#!/usr/bin/env python3
"""Build a pinned upstream core plus the reviewed per-account/source gate."""
from pathlib import Path
import os,sys,json,subprocess,shutil,tempfile
root=Path(__file__).resolve().parent.parent
go=os.environ.get('GO_BINARY','go')
version='v1.260327.1-0.20260728075948-5ca6f4b7d4dc'
meta=json.loads(subprocess.check_output([go,'mod','download','-json','github.com/xtls/xray-core@'+version],cwd=root))
if meta.get('Error'):raise RuntimeError(meta['Error'])
expected='github.com/xtls/xray-core '+version+' '+meta['Sum']
if expected not in (root/'go.sum').read_text().splitlines():raise RuntimeError('Core checksum mismatch')
work=Path(tempfile.mkdtemp(prefix='fara-core-'))
try:
 shutil.copytree(meta['Dir'],work/'src');src=work/'src'
 src.chmod(0o755)
 for p in src.rglob('*'):
  p.chmod(0o755 if p.is_dir() else 0o644)
 shutil.copytree(root/'internal/util/sessiongate',src/'app/stats/sessiongate')
 for folder in ['dispatcher','command']:
  dst=src/('app/dispatcher' if folder=='dispatcher' else 'app/stats/command')
  for p in (root/'tools/core-overlay'/folder).glob('*.go.template'):shutil.copyfile(p,dst/p.name.removesuffix('.template'))
 p=src/'app/stats/command/command.go';s=p.read_text();needle='RegisterStatsServiceServer(server, ss)';assert s.count(needle)==1;s=s.replace(needle,needle+'\n registerClientIP(server)');p.write_text(s)
 p=src/'app/dispatcher/default.go';s=p.read_text()
 s=s.replace('"github.com/xtls/xray-core/common"','"github.com/xtls/xray-core/common"\n "github.com/xtls/xray-core/app/stats/sessiongate"',1)
 for function in ['Dispatch(ctx context.Context, destination net.Destination) (*transport.Link, error)', 'DispatchLink(ctx context.Context, destination net.Destination, outbound *transport.Link) error']:
  needle='func (d *DefaultDispatcher) '+function+' {';assert s.count(needle)==1;s=s.replace(needle,needle+'\n if !relayMode {ctx=sessiongate.WithStream(ctx)}')
 old='''if p.Stats.UserOnline {
			trackOnlineIP(ctx, d.stats, user.Email, sessionInbound.Source.Address.String())
		}'''
 assert s.count(old)==1;s=s.replace(old,'trackClientIP(ctx,d.stats,user.Email,sessionInbound.Source.Address.String(),inboundLink,p.Stats.UserOnline)')
 old='''if p.Stats.UserOnline {
			trackOnlineIP(ctx, statsManager, user.Email, sessionInbound.Source.Address.String())
		}'''
 assert s.count(old)==1;s=s.replace(old,'trackClientIP(ctx,statsManager,user.Email,sessionInbound.Source.Address.String(),link,p.Stats.UserOnline)');p.write_text(s)
 p=src/'app/dispatcher/default.go';s=p.read_text()
 needle='type DefaultDispatcher struct {'
 assert s.count(needle)==1;s=s.replace(needle,needle+'\n siteProbeDirect func() (outbound.Handler,error)')
 needle='d := new(DefaultDispatcher)'
 assert s.count(needle)==1;s=s.replace(needle,needle+'\n d.siteProbeDirect=newSiteProbeDirect(ctx)')
 needle='inTag := routingLink.GetInboundTag()'
 assert s.count(needle)==1;s=s.replace(needle,needle+'\n probe := isSiteProbe(ctx,ob.Target)')
 needle='errors.LogInfo(ctx, "default route for ", destination)'
 assert s.count(needle)==1;s=s.replace(needle,needle+"""
   if probe && err != common.ErrNoClue {
    common.Close(link.Writer)
    common.Interrupt(link.Reader)
    return
   }
 """)
 old='''		handler = d.ohm.GetDefaultHandler()
'''
 new='''		if probe {
			if d.siteProbeDirect != nil {
				var err error
				handler, err = d.siteProbeDirect()
				if err != nil { errors.LogWarning(ctx, "site probe direct fallback unavailable: ",err) }
			}
		} else {
			handler = d.ohm.GetDefaultHandler()
		}
'''
 assert s.count(old)==1;s=s.replace(old,new);p.write_text(s)
 # The pinned Mux close path assumes that the dispatcher returned a bare pipe.
 p=src/'common/mux/session.go';s=p.read_text()
 old='''s.input.(*pipe.Reader).ReturnAnError(io.EOF)
		runtime.Gosched()
		// If the error set by ReturnAnError still exists, clear it.
		s.input.(*pipe.Reader).Recover()'''
 new='''reader, _ := s.input.(*pipe.Reader)
		if wrapped, ok := s.input.(interface{ XUDPPipeReader() *pipe.Reader }); ok {
			reader = wrapped.XUDPPipeReader()
		}
		if reader != nil {
			reader.ReturnAnError(io.EOF)
			runtime.Gosched()
			reader.Recover()
		} else {
			// This link cannot be detached for reuse. Expire it and let the
			// next XUDP packet dispatch a fresh link if reuse is attempted.
			common.Interrupt(s.input)
			common.Close(s.output)
		}'''
 assert s.count(old)==1;s=s.replace(old,new);p.write_text(s)
 if '--test' in sys.argv:
  dst=src/'common/mux'
  for p in (root/'tools/core-overlay/mux').glob('*.go.template'):shutil.copyfile(p,dst/p.name.removesuffix('.template'))
 p=src/'proxy/proxy.go';text=p.read_text();needle='func CopyRawConnIfExist(ctx context.Context, readerConn net.Conn, writerConn net.Conn, writer buf.Writer, timer *signal.ActivityTimer, inTimer *signal.ActivityTimer) error {';assert text.count(needle)==1
 text=text.replace('import (','import (\n "github.com/xtls/xray-core/app/stats/sessiongate"',1)
 text=text.replace(needle,needle+'\n if readerConn!=nil {defer sessiongate.AttachTransport(ctx,func(){_ =readerConn.Close()})()}')
 p.write_text(text)
 env=os.environ.copy();env['CGO_ENABLED']='1' if '--test' in sys.argv else '0'
 if '--test' in sys.argv:subprocess.run([go,'test','-race','./app/stats/sessiongate','./app/stats/command','./app/dispatcher','./common/mux'],cwd=src,env=env,check=True)
 else:
  output=Path(sys.argv[1]).resolve();output.parent.mkdir(parents=True,exist_ok=True)
  shutil.copyfile(src/'LICENSE',output.parent/'LICENSE.xray')
  (output.parent/'CORE_SOURCE.txt').write_text('Xray core '+version+'\n'+meta['Sum']+'\nSource: https://github.com/XTLS/Xray-core/tree/5ca6f4b7d4dc\nFara modifications: https://github.com/0fariid0/3x-ui/tree/v3.9.32/tools/core-overlay\nShared gate source: https://github.com/0fariid0/3x-ui/tree/v3.9.32/internal/util/sessiongate\n')
  subprocess.run([go,'build','-trimpath','-ldflags=-s -w -X github.com/xtls/xray-core/core.build=Fara-3.9.32','-o',str(output),'./main'],cwd=src,env=env,check=True)
finally:shutil.rmtree(work)
