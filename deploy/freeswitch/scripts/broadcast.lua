if not session:ready() then
  return
end

session:answer()
session:sleep(300)

local greeting = session:getVariable("greeting_sound") or ""
if greeting == "" then
  session:hangup("USER_NOT_REGISTERED")
  return
end

local beeps = freeswitch.EventConsumer("CUSTOM", "avmd::beep")
session:execute("avmd_start")

session:execute("playback", greeting)

local e = beeps:pop(1, 8000)
if e then
  session:setVariable("amd_result", "machine")
  session:execute("playback", greeting)
else
  session:setVariable("amd_result", "human")
end

session:execute("avmd_stop")
session:hangup("NORMAL_CLEARING")
