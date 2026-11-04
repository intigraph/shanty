import argparse
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
parser = argparse.ArgumentParser(description='Synthetic twelve-render cache probe; no tracker access.')
parser.add_argument('--before', type=pathlib.Path, required=True)
parser.add_argument('--after', type=pathlib.Path, required=True)
args = parser.parse_args()
results=[]
for name,source in [('installed',args.before.resolve()),('candidate',args.after.resolve())]:
 for count,exitcode in [('7','0'),('0','0'),('partial','2')]:
  with tempfile.TemporaryDirectory(prefix='shanty-inbox-demo-') as tmp:
   d=pathlib.Path(tmp);shutil.copy2(source,d/'shanty')
   (d/'st').write_text('#!/bin/sh\necho call >> "$SHANTY_TEST_CALLS"\necho "$SHANTY_TEST_COUNT"\nexit "$SHANTY_TEST_EXIT"\n');(d/'st').chmod(0o700)
   env={k:v for k,v in os.environ.items() if k in ['PATH','LANG','LC_ALL']}
   env.update(HOME=str(d),XDG_CACHE_HOME=str(d/'cache'),SHANTY_AGENT='synthetic',SHANTY_ST_BIN=str(d/'st'),SHANTY_ST_CWD=str(d),SHANTY_TEST_CALLS=str(d/'calls'),SHANTY_TEST_COUNT=count,SHANTY_TEST_EXIT=exitcode)
   outputs=[]
   # Twelve real binary renders, with private cache mtimes advanced by five
   # seconds between calls: deterministic model of t=0,5,...55, not live load.
   for tick in range(12):
    if tick:
     for p in (d/'cache').rglob('*'):
      if p.is_file():
       st=p.stat();os.utime(p,(st.st_atime,st.st_mtime-5))
    p=subprocess.run([str(d/'shanty'),'seg','inbox'],env=env,cwd=d,text=True,capture_output=True,check=True,timeout=10)
    outputs.append(p.stdout)
   calls=len((d/'calls').read_text().splitlines())
   if exitcode!='0':assert all('inbox?' in x for x in outputs),outputs
   elif count=='0':assert all(not x.strip() for x in outputs),outputs
   else:assert all('7' in x for x in outputs),outputs
   results.append(dict(binary=name,answer=count,exit=exitcode,renders=12,backend_calls=calls))
   if name=='candidate':assert calls==1,results
print(json.dumps(results,indent=2))
