package compliance

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/dnc"
)

type Decision struct {
	Eligible bool
	Reason   string
}

var Eligible = Decision{Eligible: true}

type Input struct {
	TenantID    int64
	PhoneE164   string
	Now         time.Time
	Timezone    string
	OpenHour    int
	CloseHour   int
	AllowSunday bool
	SkipHours   bool
}

type Preflight struct {
	dnc *dnc.Repo
}

func New(dncRepo *dnc.Repo) *Preflight {
	return &Preflight{dnc: dncRepo}
}

func (p *Preflight) Check(ctx context.Context, tx pgx.Tx, in Input) (Decision, error) {
	if d := hoursDecision(in); !d.Eligible {
		return d, nil
	}
	blocked, scope, err := p.dnc.IsBlockedTx(ctx, tx, in.PhoneE164)
	if err != nil {
		return Decision{}, err
	}
	if blocked {
		return Decision{Eligible: false, Reason: "dnc:" + scope}, nil
	}
	return Eligible, nil
}

func hoursDecision(in Input) Decision {
	if in.SkipHours {
		return Eligible
	}
	return checkHours(in)
}

func checkHours(in Input) Decision {
	open := in.OpenHour
	close_ := in.CloseHour
	if open == 0 && close_ == 0 {
		open = 8
		close_ = 21
	}
	if open < 0 || open > 23 || close_ < 1 || close_ > 24 || open >= close_ {
		return Decision{Eligible: false, Reason: "hours:invalid_window"}
	}

	tz := in.Timezone
	if tz == "" {
		tz = "America/New_York"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Decision{Eligible: false, Reason: "hours:bad_timezone:" + tz}
	}
	local := in.Now.In(loc)

	if !in.AllowSunday && local.Weekday() == time.Sunday {
		return Decision{Eligible: false, Reason: "hours:sunday_blocked"}
	}
	h := local.Hour()
	if h < open {
		return Decision{Eligible: false, Reason: "hours:before_open"}
	}
	if h >= close_ {
		return Decision{Eligible: false, Reason: "hours:after_close"}
	}
	return Eligible
}
