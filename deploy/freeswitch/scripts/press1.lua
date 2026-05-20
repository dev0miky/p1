if not session:ready() then
  return
end

session:answer()
session:sleep(300)

local greeting     = session:getVariable("greeting_sound")   or ""
local transfer_to  = session:getVariable("transfer_to")      or ""
local bridge_d     = session:getVariable("bridge_digit")     or "1"
local opt_out_d    = session:getVariable("opt_out_digit")    or ""
local wait_ms_str  = session:getVariable("wait_timeout_ms")  or "8000"
local pre_bridge   = session:getVariable("pre_bridge_sound") or ""

if greeting == "" then
  session:hangup("USER_NOT_REGISTERED")
  return
end

local wait_ms = tonumber(wait_ms_str) or 8000

session:execute("record_session", "/recordings/" .. session:get_uuid() .. ".wav")
session:execute("avmd_start")
session:execute("spandsp_start_dtmf")

local valid = bridge_d
if opt_out_d ~= "" then
  valid = valid .. opt_out_d
end

session:setVariable("playback_terminators", valid .. "#")
session:execute("playback", greeting)

local digit = session:getVariable("playback_terminator_used") or ""

if digit == "" then
  session:execute("read", string.format("0 1 silence_stream://%d|0 dialer_digit %d %s",
    wait_ms, wait_ms, valid .. "#"))
  digit = session:getVariable("dialer_digit") or ""
end

if digit == bridge_d then
  if transfer_to == "" then
    session:setVariable("press1_action", "no_transfer_target")
    session:hangup("CALL_REJECTED")
    return
  end
  if pre_bridge ~= "" then
    session:execute("playback", pre_bridge)
  end
  session:setVariable("press1_action", "transfer")
  session:execute("bridge", transfer_to)
elseif opt_out_d ~= "" and digit == opt_out_d then
  session:setVariable("press1_action", "opt_out")
  session:hangup("USER_REFUSE")
else
  session:setVariable("press1_action", "no_input")
  session:hangup("ALLOTTED_TIMEOUT")
end
