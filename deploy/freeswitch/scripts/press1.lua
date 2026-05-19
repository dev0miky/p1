if not session:ready() then
  return
end

session:answer()
session:sleep(300)

local greeting = session:getVariable("greeting_sound") or ""
local transfer_to = session:getVariable("transfer_to") or ""

if greeting == "" then
  session:hangup("USER_NOT_REGISTERED")
  return
end

session:setVariable("playback_terminators", "19#")
session:execute("playback", greeting)

local digit = session:getVariable("playback_terminator_used") or ""

if digit == "" then
  session:execute("read", "0 1 silence_stream://6000|0 dialer_digit 6000 19#")
  digit = session:getVariable("dialer_digit") or ""
end

if digit == "1" then
  if transfer_to == "" then
    session:setVariable("press1_action", "no_transfer_target")
    session:hangup("CALL_REJECTED")
    return
  end
  local pre_bridge = session:getVariable("pre_bridge_sound") or ""
  if pre_bridge ~= "" then
    session:execute("playback", pre_bridge)
  end
  session:setVariable("press1_action", "transfer")
  session:execute("bridge", transfer_to)
elseif digit == "9" then
  session:setVariable("press1_action", "opt_out")
  session:hangup("USER_REFUSE")
else
  session:setVariable("press1_action", "no_input")
  session:hangup("ALLOTTED_TIMEOUT")
end
