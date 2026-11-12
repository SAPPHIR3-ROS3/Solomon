import argparse
import socket,struct,json,time
from pathlib import Path
from gi.repository import GLib
parser=argparse.ArgumentParser()
parser.add_argument('--url')
parser.add_argument('--live-title')
parser.add_argument('--original-blur',action='store_true')
parser.add_argument('--replay-offset',type=int)
parser.add_argument('--chat-id')
parser.add_argument('--sse-replay',action='store_true')
parser.add_argument('--highlight-control',action='store_true')
parser.add_argument('--chunk-size',type=int,default=12)
parser.add_argument('--chunk-interval-ms',type=int,default=50)
parser.add_argument('--output',required=True)
parser.add_argument('--duration-ms',type=int,default=12000)
parser.add_argument('--port',type=int,default=39221)
args=parser.parse_args()
if not args.url and not args.live_title:parser.error('--url or --live-title is required')
if args.replay_offset is not None and (not args.live_title or not args.chat_id or args.replay_offset<0):parser.error('Replay requires --live-title, --chat-id and a nonnegative offset')
if args.chunk_size<1 or args.chunk_interval_ms<1:parser.error('Chunk size and interval must be positive')
if not 1 <= args.port <= 65535 or not 100 <= args.duration_ms <= 120000:
 parser.error('Invalid port or recording duration')
sock=socket.create_connection(('127.0.0.1',args.port),timeout=10)
def send(name,signature,values):
 body=name.encode()+b'\0'+GLib.Variant(signature,values).get_data_as_bytes().get_data()
 sock.sendall(struct.pack('!I',len(body))+b'\x01'+body)
def exact(size):
 data=b''
 while len(data)<size:
  chunk=sock.recv(size-len(data))
  if not chunk:raise RuntimeError('Inspector connection closed')
  data+=chunk
 return data
signatures={'DidSetupInspectorClient':'(ay)','SetTargetList':'(ta(tsssb))','SendMessageToFrontend':'(tts)'}
def receive():
 size=struct.unpack('!I',exact(4))[0]
 if not 2 <= size <= 64*1024*1024:raise RuntimeError('Invalid inspector message size')
 flags=exact(1);body=exact(size);name,data=body.split(b'\0',1)
 name=name.decode()
 return name,GLib.Variant.new_from_bytes(GLib.VariantType.new(signatures[name]),GLib.Bytes.new(data),False).unpack() if name in signatures else None
send('SetupInspectorClient','(ay)',([0],))
while True:
 name,params=receive()
 if name=='SetTargetList' and params[1]:
  connection=params[0];target=next(x[0] for x in params[1] if x[1]=='WebPage' and not x[3].startswith('resource:'));break
send('Setup','(tt)',(connection,target))
sequence=0;events=[]
while True:
 name,payload=receive()
 if name=='SendMessageToFrontend':
  message=json.loads(payload[2]);events.append(message)
  if message.get('method')=='Target.targetCreated':
   page_target=message['params']['targetInfo']['targetId'];break
def command(method,params=None):
 global sequence
 sequence+=1;request_id=sequence
 send('SendMessageToBackend','(tts)',(connection,target,json.dumps({'id':request_id+100000,'method':'Target.sendMessageToTarget','params':{'targetId':page_target,'message':json.dumps({'id':request_id,'method':method,'params':params or {}})}})))
 while True:
  name,payload=receive()
  if name!='SendMessageToFrontend':continue
  message=json.loads(payload[2])
  if message.get('id')==request_id+100000 and 'error' in message:raise RuntimeError(str(message))
  if message.get('method')=='Target.dispatchMessageFromTarget':message=json.loads(message['params']['message'])
  if message.get('id')==request_id:return message
  events.append(message)
def evaluate(expression):
 result=command('Runtime.evaluate',{'expression':expression,'returnByValue':True})
 if 'error' in result or result.get('result',{}).get('wasThrown'):raise RuntimeError(str(result))
 return result.get('result',{}).get('result',{}).get('value',result)

if args.live_title:
 evaluate("""document.querySelector('button[aria-label="Expand side panel"]')?.click();true""")
 title=json.dumps(args.live_title)
 deadline=time.monotonic()+20
 while time.monotonic()<deadline:
  if evaluate("[...document.querySelectorAll('button')].some(button=>button.textContent.startsWith("+title+"))"):break
  time.sleep(.2)
 else:raise RuntimeError('The requested chat is not visible in the sidebar')
 evaluate("[...document.querySelectorAll('button')].find(button=>button.textContent.startsWith("+title+")).click();true")
 deadline=time.monotonic()+20
 while time.monotonic()<deadline:
  if evaluate('!!document.querySelector(".chat-view canvas.chat-floating-letters")'):break
  time.sleep(.2)
 else:raise RuntimeError('Live chat did not load with floating letters')
 evaluate("""document.querySelector('button[aria-label="Collapse side panel"]')?.click();true""")
 if args.original_blur:
  evaluate("""const style=document.createElement('style');style.textContent='.chat-composer-background{backdrop-filter:blur(18px)!important;-webkit-backdrop-filter:blur(18px)!important;background:color-mix(in srgb,var(--color-surface-raised) 28%,transparent)!important}';document.head.append(style);true""")
 evaluate("""window.__errors=[];window.startProbe=()=>{window.samples=[];window.probeRunning=true;let previous=0;const generation=window.probeGeneration=(window.probeGeneration??0)+1;function sample(now){if(!window.probeRunning||window.probeGeneration!==generation)return;const shell=document.querySelector('.chat-messages-shell');const response=document.querySelector('.chat-message.is-assistant .chat-response-content');const rect=response?.getBoundingClientRect();window.samples.push({time:now,gap:previous?now-previous:0,height:rect?.height,top:rect?.top,length:response?.textContent.length,htmlLength:response?.innerHTML.length,scrollTop:shell?.scrollTop,scrollHeight:shell?.scrollHeight});previous=now;requestAnimationFrame(sample)}requestAnimationFrame(sample)};true""")
 if args.replay_offset is not None:
  if not args.chat_id:parser.error('--chat-id is required for an in-memory replay')
  evaluate("import('/src/chat/chatStore.ts').then(module=>{window.__inspectorStore=module;window.__inspectorReady=true});true")
  deadline=time.monotonic()+10
  while time.monotonic()<deadline:
   if evaluate('window.__inspectorReady===true'):break
   time.sleep(.1)
  else:raise RuntimeError('Unable to access the diagnostic window chat store')
  if args.sse_replay:
   source=Path(__file__).with_name('replay.js').read_text()
   evaluate('('+source+')('+json.dumps({'chatID':args.chat_id,'chunkSize':args.chunk_size,'intervalMs':args.chunk_interval_ms,'highlightControl':args.highlight_control})+');true')
   deadline=time.monotonic()+10
   while time.monotonic()<deadline:
    if evaluate('window.__inspectorReplayReady===true'):break
    time.sleep(.1)
   else:raise RuntimeError('SSE replay initialization failed')
  else:
   evaluate("""window.startStream=()=>{const store=window.__inspectorStore;const chat=store.getChat(%s);if(!chat)throw new Error('Diagnostic chat is not cached');window.__inspectorOriginalChat=chat;const index=chat.messages.findIndex(message=>message.role==='assistant'&&message.content.length>0);if(index<0)throw new Error('Recorded response not found');const source=chat.messages[index].content;const publish=content=>store.updateChat(chat.id,current=>({...current,messages:current.messages.map((message,i)=>i===index?{...message,content}:message)}));publish(source.slice(0,%d));window.__inspectorReplay={sourceLength:source.length,received:%d,networkDone:false};let cursor=%d;const timer=setInterval(()=>{cursor=Math.min(source.length,cursor+%d);publish(source.slice(0,cursor));window.__inspectorReplay.received=cursor;if(cursor===source.length){clearInterval(timer);window.__inspectorReplay.networkDone=true}},%d)};true""" % (json.dumps(args.chat_id),args.replay_offset,args.replay_offset,args.replay_offset,args.chunk_size,args.chunk_interval_ms))
 time.sleep(2)
if args.url:
 evaluate("""(()=>{const frame=document.createElement('iframe');frame.style.display='none';document.body.append(frame);return frame.contentWindow.fetch(%s).then(r=>r.text()).then(html=>{document.open();document.write(html.replaceAll('="/','="'+%s));document.close()})})();true""" % (json.dumps(args.url), json.dumps(args.url)))
 deadline=time.monotonic()+20
 while time.monotonic()<deadline:
  try:
   if evaluate('typeof window.startProbe === "function" && !!document.querySelector(".chat-view") && !!document.querySelector("canvas.chat-floating-letters")'):break
  except RuntimeError:pass
  time.sleep(.1)
 else:raise RuntimeError('Diagnostic fixture did not load: '+str(evaluate('JSON.stringify({url:location.href,body:document.body?.innerText.slice(0,300),errors:window.__errors})'))) 
 time.sleep(1.5)
metadata=evaluate('JSON.stringify({url:location.href,title:document.title,agent:navigator.userAgent,viewport:[innerWidth,innerHeight],canvas:!!document.querySelector("canvas.chat-floating-letters"),blur:document.querySelector(".chat-composer-background")?getComputedStyle(document.querySelector(".chat-composer-background")).backdropFilter:null})')
print(metadata,flush=True)
command('Page.enable')
command('Timeline.start',{'maxCallStackDepth':3})
cases=[]
for mode in (['streaming'] if args.replay_offset is not None else ['idle'] if args.live_title else ['idle','streaming']):
 print(evaluate(('window.startStream();' if mode=='streaming' else '')+'window.startProbe();true'),flush=True)
 deadline=time.monotonic()+args.duration_ms/1000
 while time.monotonic()<deadline:
  sock.settimeout(max(0.1,deadline-time.monotonic()))
  try:
   name,payload=receive()
   if name=='SendMessageToFrontend':events.append(json.loads(payload[2]))
  except socket.timeout:break
 sock.settimeout(10)
 data=json.loads(evaluate('window.probeRunning=false;JSON.stringify({samples:window.samples,errors:window.__errors})'))
 cases.append({'mode':mode,**data})
 print(mode,len(data['samples']),flush=True)
command('Timeline.stop')
snapshot=command('Page.snapshotRect',{'x':0,'y':0,'width':1280,'height':840,'coordinateSystem':'Viewport'})
if snapshot.get('result',{}).get('dataURL'):
 import base64
 Path(args.output).with_suffix('.png').write_bytes(base64.b64decode(snapshot['result']['dataURL'].split(',',1)[1]))
Path(args.output).write_text(json.dumps({'metadata':json.loads(metadata),'cases':cases,'replay':evaluate('JSON.stringify(window.__inspectorReplay??null)'),'events':events},indent=2))
if args.replay_offset is not None:evaluate('if(window.__inspectorCleanup)window.__inspectorCleanup();else window.__inspectorStore.updateChat(window.__inspectorOriginalChat.id,()=>window.__inspectorOriginalChat);true')
send('FrontendDidClose','(tt)',(connection,target));sock.close()
