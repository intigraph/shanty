"""Actual private 170-column tmux; deterministic segment text, no board reads.

Requires baseline/candidate binaries in SHANTY_DEMO_BASELINE/CANDIDATE.
"""
import os,fcntl,termios,struct,pty,subprocess,tempfile,time,select,re,json
from pathlib import Path
import pyte
class CaptureScreen(pyte.Screen):
 def report_device_status(self, *args, **kwargs): pass
 def report_device_attributes(self, *args, **kwargs): pass
results=[]
for label in ['baseline','candidate']:
 with tempfile.TemporaryDirectory() as d:
  root=Path(d);config=root/'config'/'shanty';config.mkdir(parents=True)
  (config/'config.toml').write_text('[status]\nleft=["session","crewid"]\nright=["task","cpu","clock"]\n')
  env=os.environ.copy();env.update(HOME=d,XDG_CONFIG_HOME=str(root/'config'),PATH='/usr/bin:/bin',SHANTY_TMUX_SOCKET='tim-cobi5q-private-absent',TERM='xterm-256color')
  r=subprocess.run([os.environ['SHANTY_DEMO_'+label.upper()],'apply'],env=env,capture_output=True,text=True)
  assert r.returncode==1 and 'no tmux server' in r.stderr
  raw=(config/'tmux.conf').read_text()
  def replace(match):
   name=match.group(1)
   return {'crewid':'ROLE:lead BUSY settings:STALE','session':'fixture-agent','task':'task-'+'X'*220}.get(name,name)
  rendered=re.sub(r'#\([^)]* seg (\w+)(?: #\{session_name\})?\)',replace,raw)
  cfg=root/'fixture.conf';cfg.write_text(rendered)
  sock='tim-cobi5q-'+label+'-'+str(os.getpid())
  subprocess.run(['/usr/bin/tmux','-L',sock,'-f',str(cfg),'new-session','-d','-s','fixture','-x','170','-y','24','sleep 30'],env=env,check=True,capture_output=True)
  pid,fd=pty.fork()
  if pid==0:
   fcntl.ioctl(0,termios.TIOCSWINSZ,struct.pack('HHHH',24,170,0,0))
   os.execve('/usr/bin/tmux',['tmux','-L',sock,'attach-session','-t','fixture'],env)
  screen=CaptureScreen(170,24);stream=pyte.Stream(screen)
  fcntl.ioctl(fd,termios.TIOCSWINSZ,struct.pack('HHHH',24,170,0,0))
  try:
   until=time.monotonic()+2
   while time.monotonic()<until:
    if select.select([fd],[],[],0.1)[0]:
     stream.feed(os.read(fd,65536).decode(errors='replace'))
   row=screen.display[-1]
   results.append({'source':label,'width':170,'role_visible':'ROLE:lead' in row,'status_row':row.rstrip()})
   os.write(fd,b'\x01d')
  finally:
   subprocess.run(['/usr/bin/tmux','-L',sock,'kill-server'],env=env,capture_output=True)
   os.close(fd);os.waitpid(pid,0)
Path(__file__).with_name('render170.json').write_text(json.dumps(results,indent=2));print(json.dumps(results,indent=2))
assert not results[0]['role_visible'] and results[1]['role_visible']
