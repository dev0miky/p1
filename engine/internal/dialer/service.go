package dialer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/callerid"
	"p1/engine/internal/campaign"
	"p1/engine/internal/compliance"
	"p1/engine/internal/db"
	"p1/engine/internal/dnc"
	"p1/engine/internal/esl"
	"p1/engine/internal/fsm"
	"p1/engine/internal/lead"
)

type Config struct {
	Pool             *pgxpool.Pool
	ESL              *esl.Client
	NodeID           string
	GatewayName      string
	ForceDest        string
	TestPlayback     string
	SoundRoot        string
	OriginateTimeout time.Duration
	TickInterval     time.Duration
	JanitorInterval  time.Duration
	MaxConcurrent    int
	Logger           *slog.Logger
}

type Service struct {
	cfg      Config
	campaign *campaign.Repo
	lead     *lead.Repo
	fsm      *fsm.Repo
	cid      *callerid.Repo
	dnc      *dnc.Repo
	pre      *compliance.Preflight
	logger   *slog.Logger

	mu       sync.Mutex
	uuidLead map[string]int64
	uuidTen  map[string]int64
}

func New(cfg Config) *Service {
	if cfg.NodeID == "" {
		cfg.NodeID = "dialer-" + uuid.NewString()[:8]
	}
	if cfg.OriginateTimeout <= 0 {
		cfg.OriginateTimeout = 30 * time.Second
	}
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 500 * time.Millisecond
	}
	if cfg.JanitorInterval <= 0 {
		cfg.JanitorInterval = 30 * time.Second
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 50
	}
	if cfg.GatewayName == "" {
		cfg.GatewayName = "loopback"
	}
	if cfg.SoundRoot == "" {
		cfg.SoundRoot = "/data/sounds"
	}
	dncRepo := dnc.NewRepo()
	return &Service{
		cfg:      cfg,
		campaign: campaign.NewRepo(),
		lead:     lead.NewRepo(),
		fsm:      fsm.NewRepo(),
		cid:      callerid.NewRepo(),
		dnc:      dncRepo,
		pre:      compliance.New(dncRepo),
		logger:   cfg.Logger,
		uuidLead: make(map[string]int64),
		uuidTen:  make(map[string]int64),
	}
}

func (s *Service) Run(ctx context.Context) error {
	s.cfg.ESL.OnEvent(func(e esl.Event) { s.handleEvent(ctx, e) })

	if err := s.cfg.ESL.Subscribe(ctx,
		"CHANNEL_CREATE",
		"CHANNEL_ANSWER",
		"CHANNEL_BRIDGE",
		"CHANNEL_HANGUP_COMPLETE",
		"BACKGROUND_JOB",
		"DTMF",
		"CUSTOM avmd::beep",
	); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	tick := time.NewTicker(s.cfg.TickInterval)
	defer tick.Stop()
	janitor := time.NewTicker(s.cfg.JanitorInterval)
	defer janitor.Stop()

	s.logger.Info("dialer service running",
		"node", s.cfg.NodeID,
		"tick_ms", s.cfg.TickInterval.Milliseconds(),
		"gateway", s.cfg.GatewayName,
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			s.tick(ctx)
		case <-janitor.C:
			s.runJanitor(ctx)
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	var campaigns []campaign.Campaign
	if err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		var err error
		campaigns, err = s.campaign.ListActiveTx(ctx, tx)
		return err
	}); err != nil {
		s.logger.Error("list active campaigns", "err", err)
		return
	}
	for _, c := range campaigns {
		s.paceCampaign(ctx, c)
	}
}

func (s *Service) paceCampaign(ctx context.Context, c campaign.Campaign) {
	var inFlight int
	if err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM call_state
			WHERE campaign_id = $1
			  AND state NOT IN ('completed','failed','no_answer','busy','voicemail','opt_out')
		`, c.ID).Scan(&inFlight)
	}); err != nil {
		s.logger.Error("count inflight", "campaign", c.ID, "err", err)
		return
	}

	toDial := DecideToDial(PacingInput{
		Mode:          string(c.Mode),
		DialRatio:     c.DialRatio,
		MaxConcurrent: s.cfg.MaxConcurrent,
		InFlight:      inFlight,
	})
	if toDial <= 0 {
		return
	}

	var claimed []lead.Lead
	if err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin", TenantID: c.TenantID}, func(tx pgx.Tx) error {
		var err error
		claimed, err = s.lead.ClaimBatchTx(ctx, tx, lead.ClaimOptions{
			CampaignID: c.ID,
			NodeID:     s.cfg.NodeID,
			Limit:      toDial,
			LockFor:    2 * time.Minute,
			Constraint: c.CallConstraint,
		})
		return err
	}); err != nil {
		s.logger.Error("claim leads", "campaign", c.ID, "err", err)
		return
	}
	if len(claimed) == 0 {
		return
	}
	s.logger.Info("paced", "campaign", c.ID, "in_flight", inFlight, "to_dial", toDial, "claimed", len(claimed))

	for _, l := range claimed {
		s.originate(ctx, c, l)
	}
}

func (s *Service) originate(ctx context.Context, c campaign.Campaign, l lead.Lead) {
	tz := "America/New_York"
	if l.Timezone != nil && *l.Timezone != "" {
		tz = *l.Timezone
	}

	hours := c.ParseCallingHours()
	var decision compliance.Decision
	err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin", TenantID: c.TenantID}, func(tx pgx.Tx) error {
		var err error
		decision, err = s.pre.Check(ctx, tx, compliance.Input{
			TenantID:    c.TenantID,
			PhoneE164:   l.PhoneE164,
			Now:         time.Now(),
			Timezone:    tz,
			OpenHour:    hours.OpenHour,
			CloseHour:   hours.CloseHour,
			AllowSunday: hours.AllowSunday,
			SkipHours:   !hours.Enabled,
		})
		return err
	})
	if err != nil {
		s.logger.Error("preflight", "lead", l.ID, "err", err)
		s.markLeadFailed(ctx, c.TenantID, l.ID, "preflight_error")
		return
	}
	if !decision.Eligible {
		s.logger.Info("preflight blocked", "lead", l.ID, "phone", l.PhoneE164, "reason", decision.Reason)
		s.markLeadOutcome(ctx, c.TenantID, l.ID, leadStatusFromReason(decision.Reason), retryAfterFromReason(decision.Reason))
		return
	}

	callUUID := uuid.NewString()
	if err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin", TenantID: c.TenantID}, func(tx pgx.Tx) error {
		_, err := s.fsm.CreateTx(ctx, tx, fsm.CreateInput{
			UUID:          callUUID,
			TenantID:      c.TenantID,
			CampaignID:    &c.ID,
			CampaignRunNo: c.RunNo,
			LeadID:        &l.ID,
			DialedNumber:  l.PhoneE164,
		})
		return err
	}); err != nil {
		s.logger.Error("create call_state", "lead", l.ID, "err", err)
		s.markLeadFailed(ctx, c.TenantID, l.ID, "create_state_error")
		return
	}

	s.mu.Lock()
	s.uuidLead[callUUID] = l.ID
	s.uuidTen[callUUID] = c.TenantID
	s.mu.Unlock()

	callerNumber, callerName := s.pickCallerID(ctx, c.TenantID, c.ID, l.Attempts)
	flow := s.pickScriptFlow(ctx, c.TenantID, c.ID)

	vars := esl.OriginateVars{
		"origination_uuid":             callUUID,
		"origination_caller_id_number": callerNumber,
		"tenant_id":                    strconv.FormatInt(c.TenantID, 10),
		"campaign_id":                  strconv.FormatInt(c.ID, 10),
		"lead_id":                      strconv.FormatInt(l.ID, 10),
		"ignore_early_media":           "true",
		"originate_timeout":            strconv.Itoa(int(s.cfg.OriginateTimeout.Seconds())),
	}
	if callerName != "" {
		vars["origination_caller_id_name"] = callerName
	}

	dest := l.PhoneE164
	if l.DialDestination != nil && *l.DialDestination != "" {
		dest = *l.DialDestination
	} else if s.cfg.ForceDest != "" {
		dest = s.cfg.ForceDest
	}
	action := "&park"
	switch flow.kind {
	case "press1":
		vars["greeting_sound"] = flow.greetingPath
		vars["transfer_to"] = flow.transferTo
		vars["bridge_digit"] = flow.bridgeDigit
		vars["wait_timeout_ms"] = strconv.Itoa(flow.waitTimeoutMS)
		if flow.optOutDigit != "" {
			vars["opt_out_digit"] = flow.optOutDigit
		}
		if flow.preBridgePath != "" {
			vars["pre_bridge_sound"] = flow.preBridgePath
		}
		action = "&lua(press1.lua)"
	case "broadcast":
		vars["greeting_sound"] = flow.greetingPath
		action = "&lua(broadcast.lua)"
	default:
		if s.cfg.TestPlayback != "" {
			action = "&playback(" + s.cfg.TestPlayback + ")"
		}
	}
	octx, cancel := context.WithTimeout(ctx, 10*time.Second)
	jobUUID, err := s.cfg.ESL.Originate(octx, esl.OriginateParams{
		Vars:    vars,
		Gateway: s.cfg.GatewayName,
		Dest:    dest,
		Action:  action,
	})
	cancel()
	if err != nil {
		s.logger.Error("originate", "lead", l.ID, "uuid", callUUID, "err", err)
		s.failCall(ctx, c.TenantID, callUUID, "originate_error")
		s.markLeadFailed(ctx, c.TenantID, l.ID, "originate_error")
		return
	}

	if err := s.advanceState(ctx, c.TenantID, callUUID, fsm.StateOriginating, "bgapi_accepted", nil); err != nil {
		s.logger.Warn("advance to originating", "uuid", callUUID, "err", err)
	}

	s.logger.Info("originated",
		"call_uuid", callUUID,
		"job_uuid", jobUUID,
		"tenant", c.TenantID,
		"campaign", c.ID,
		"lead", l.ID,
		"phone", l.PhoneE164,
	)
}

func (s *Service) handleEvent(ctx context.Context, e esl.Event) {
	advance, ok := EventToTransition(e)
	if !ok {
		return
	}

	tenantID := s.tenantForUUID(advance.UUID)
	if tenantID == 0 {
		return
	}

	if err := s.advanceState(ctx, tenantID, advance.UUID, advance.To, advance.Reason, &advance); err != nil {
		if errors.Is(err, fsm.ErrInvalidTransition) || errors.Is(err, fsm.ErrVersionConflict) {
			s.logger.Debug("transition skipped", "uuid", advance.UUID, "to", advance.To, "err", err)
			return
		}
		s.logger.Error("transition", "uuid", advance.UUID, "to", advance.To, "err", err)
		return
	}

	if advance.To.IsTerminal() {
		s.finalizeCall(ctx, tenantID, advance.UUID, advance.To, advance.HangupCause)
	}
}

func (s *Service) advanceState(ctx context.Context, tenantID int64, uuid string, to fsm.State, reason string, adv *StateAdvance) error {
	return db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		current, err := s.fsm.GetTx(ctx, tx, uuid)
		if err != nil {
			return err
		}
		in := fsm.TransitionInput{
			UUID:        uuid,
			FromVersion: current.Version,
			To:          to,
			Reason:      reason,
		}
		if adv != nil {
			in.StampAnswered = adv.StampAnswered
			in.StampEnded = adv.StampEnded
			if adv.HangupCause != "" {
				hc := adv.HangupCause
				in.HangupCause = &hc
			}
		}
		if _, err := s.fsm.TransitionTx(ctx, tx, in); err != nil {
			return err
		}
		if current.LeadID != nil {
			delta := counterDeltaFor(to)
			if !delta.IsZero() {
				if err := s.lead.IncrementCountersTx(ctx, tx, *current.LeadID, delta); err != nil {
					return err
				}
			}
		}
		if to == fsm.StateOptOut && current.LeadID != nil {
			if err := s.writeOptOutDNCTx(ctx, tx, tenantID, *current.LeadID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) writeOptOutDNCTx(ctx context.Context, tx pgx.Tx, tenantID, leadID int64) error {
	l, err := s.lead.GetLeadTx(ctx, tx, leadID)
	if err != nil {
		return err
	}
	tenant := tenantID
	source := "dtmf_optout"
	reason := "press 9 in press-1 ivr"
	_, err = s.dnc.AddInternalTx(ctx, tx, dnc.Entry{
		TenantID:  &tenant,
		PhoneE164: l.PhoneE164,
		Source:    &source,
		Reason:    &reason,
	})
	if err != nil && !isUniqueViolation(err) {
		return err
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505")
}

func counterDeltaFor(to fsm.State) lead.CounterDelta {
	switch to {
	case fsm.StateOriginating:
		return lead.CounterDelta{NCalls: 1, SetCallTimes: true}
	case fsm.StateRinging:
		return lead.CounterDelta{NRinged: 1}
	case fsm.StateAnswered:
		return lead.CounterDelta{NAnswered: 1}
	case fsm.StateBridging:
		return lead.CounterDelta{NTransferred: 1}
	case fsm.StateBridged:
		return lead.CounterDelta{NTransferCompleted: 1}
	case fsm.StateFailed:
		return lead.CounterDelta{NError: 1}
	case fsm.StateVoicemail:
		return lead.CounterDelta{NVoicemail: 1}
	case fsm.StateOptOut:
		return lead.CounterDelta{NWentToDNC: 1}
	}
	return lead.CounterDelta{}
}

// pickCallerID selects a caller_id from the campaign's attached pool using
// the lead's attempt count as a round-robin index. Returns the E.164 number
// and the display name. If the pool is empty, returns a placeholder + logs
// at warn level so the operator notices missing config.
func (s *Service) pickCallerID(ctx context.Context, tenantID, campaignID int64, attemptIdx int) (string, string) {
	var pool []callerid.CallerID
	err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		var e error
		pool, e = s.cid.ListForCampaignTx(ctx, tx, campaignID)
		return e
	})
	if err != nil {
		s.logger.Warn("caller_id pool lookup failed", "err", err, "campaign", campaignID)
		return "+10000000000", ""
	}
	if len(pool) == 0 {
		s.logger.Warn("no caller_ids attached to campaign — using placeholder; carriers with STIR/SHAKEN will reject", "campaign", campaignID)
		return "+10000000000", ""
	}
	idx := attemptIdx
	if idx < 0 {
		idx = 0
	}
	pick := pool[idx%len(pool)]
	displayName := ""
	if pick.DisplayName != nil {
		displayName = *pick.DisplayName
	}
	return pick.E164Number, displayName
}

type scriptFlow struct {
	kind          string
	transferTo    string
	greetingPath  string
	preBridgePath string
	bridgeDigit   string
	waitTimeoutMS int
	optOutDigit   string
}

func (s *Service) pickScriptFlow(ctx context.Context, tenantID, campaignID int64) scriptFlow {
	var scripts []campaign.AttachedScript
	err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		var e error
		scripts, e = s.campaign.ListAttachedScriptsTx(ctx, tx, campaignID)
		return e
	})
	if err != nil {
		s.logger.Warn("script lookup failed", "campaign", campaignID, "err", err)
		return scriptFlow{}
	}
	for i := range scripts {
		sc := &scripts[i]
		if sc.GreetingFileKey == nil || sc.GreetingTenantID == nil {
			continue
		}
		greeting := s.soundPath(*sc.GreetingTenantID, *sc.GreetingFileKey)
		switch sc.Type {
		case "broadcast":
			return scriptFlow{kind: "broadcast", greetingPath: greeting}
		case "press1":
			target := resolveBridgeTarget(sc)
			if target == "" {
				continue
			}
			f := scriptFlow{
				kind:          "press1",
				transferTo:    target,
				greetingPath:  greeting,
				bridgeDigit:   sc.BridgeDigit,
				waitTimeoutMS: sc.WaitTimeoutMS,
			}
			if f.bridgeDigit == "" {
				f.bridgeDigit = "1"
			}
			if f.waitTimeoutMS <= 0 {
				f.waitTimeoutMS = 8000
			}
			if sc.OptOutDigit != nil {
				f.optOutDigit = *sc.OptOutDigit
			}
			if sc.PreBridgeFileKey != nil && sc.PreBridgeTenantID != nil {
				f.preBridgePath = s.soundPath(*sc.PreBridgeTenantID, *sc.PreBridgeFileKey)
			}
			return f
		}
	}
	return scriptFlow{}
}

func (s *Service) soundPath(tenantID int64, fileKey string) string {
	return s.cfg.SoundRoot + "/" + strconv.FormatInt(tenantID, 10) + "/" + fileKey
}

func resolveBridgeTarget(sc *campaign.AttachedScript) string {
	if sc.AgentDialString != nil && *sc.AgentDialString != "" {
		return *sc.AgentDialString
	}
	if sc.TransferTo != nil && *sc.TransferTo != "" {
		return *sc.TransferTo
	}
	return ""
}

func (s *Service) failCall(ctx context.Context, tenantID int64, uuid, reason string) {
	if err := s.advanceState(ctx, tenantID, uuid, fsm.StateFailed, reason, nil); err != nil {
		s.logger.Error("fail call", "uuid", uuid, "err", err)
	}
}

func (s *Service) finalizeCall(ctx context.Context, tenantID int64, callUUID string, to fsm.State, hangupCause string) {
	s.mu.Lock()
	leadID := s.uuidLead[callUUID]
	delete(s.uuidLead, callUUID)
	delete(s.uuidTen, callUUID)
	s.mu.Unlock()
	if leadID == 0 {
		return
	}
	status := leadStatusFromOutcome(to, hangupCause)
	s.markLeadOutcome(ctx, tenantID, leadID, status, nil)
}

func (s *Service) tenantForUUID(uuid string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.uuidTen[uuid]
}

func (s *Service) markLeadOutcome(ctx context.Context, tenantID, leadID int64, status string, retry *time.Time) {
	if err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		return s.lead.MarkLeadOutcomeTx(ctx, tx, leadID, status, retry)
	}); err != nil {
		s.logger.Error("mark lead outcome", "lead", leadID, "err", err)
	}
}

func (s *Service) markLeadFailed(ctx context.Context, tenantID, leadID int64, reason string) {
	s.markLeadOutcome(ctx, tenantID, leadID, "failed", nil)
	s.logger.Debug("lead failed", "lead", leadID, "reason", reason)
}

func (s *Service) runJanitor(ctx context.Context) {
	if err := db.WithCtx(ctx, s.cfg.Pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		n, err := s.lead.ReleaseExpiredLocksTx(ctx, tx)
		if err != nil {
			return err
		}
		if n > 0 {
			s.logger.Info("released expired lead locks", "n", n)
		}
		return nil
	}); err != nil {
		s.logger.Error("janitor", "err", err)
	}
}

func leadStatusFromReason(reason string) string {
	switch {
	case len(reason) >= 4 && reason[:4] == "dnc:":
		return "dnc"
	case len(reason) >= 6 && reason[:6] == "hours:":
		return "queued"
	default:
		return "failed"
	}
}

func retryAfterFromReason(reason string) *time.Time {
	if len(reason) >= 6 && reason[:6] == "hours:" {
		t := time.Now().Add(2 * time.Hour)
		return &t
	}
	return nil
}

func leadStatusFromOutcome(to fsm.State, _ string) string {
	switch to {
	case fsm.StateCompleted:
		return "done"
	case fsm.StateNoAnswer, fsm.StateBusy:
		return "queued"
	case fsm.StateVoicemail:
		return "done"
	case fsm.StateOptOut:
		return "opt_out"
	default:
		return "failed"
	}
}
