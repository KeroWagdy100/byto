import { useEffect, useState } from 'react';
import { CheckCircle2, XCircle, Loader2, Download } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from './ui/dialog';
import { Button } from './ui/button';
import { CheckYtDlp, DownloadYtDlp, CheckFfmpeg, DownloadFfmpeg } from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

interface Dependency {
  name: string;
  status: 'checking' | 'found' | 'missing' | 'downloading';
  version?: string;
  progress?: number;
}

interface DependencyCheckDialogProps {
  onClose: () => void;
}

export function DependencyCheckDialog({ onClose }: DependencyCheckDialogProps) {
  const [dependencies, setDependencies] = useState<Dependency[]>([
    { name: 'yt-dlp', status: 'checking' },
    { name: 'ffmpeg', status: 'checking' }
  ]);

  useEffect(() => {
    checkDependencies();
  }, []);

  useEffect(() => {
    const unsubYtDlp = EventsOn('ytdlp_download_progress', (data: { downloaded: number; total: number; percentage: number }) => {
      setDependencies(prev => prev.map(dep =>
        dep.name === 'yt-dlp' ? { ...dep, progress: Math.round(data.percentage) } : dep
      ));
    });
    const unsubFfmpeg = EventsOn('ffmpeg_download_progress', (data: { downloaded: number; total: number; percentage: number }) => {
      setDependencies(prev => prev.map(dep =>
        dep.name === 'ffmpeg' ? { ...dep, progress: Math.round(data.percentage) } : dep
      ));
    });
    return () => {
      EventsOff('ytdlp_download_progress');
      EventsOff('ffmpeg_download_progress');
    };
  }, []);

  const checkDependencies = async () => {
    let ytDlpInstalled = false;
    let ffmpegInstalled = false;
    // yt-dlp
    try {
      const result = await CheckYtDlp();
      ytDlpInstalled = !!result.installed;
      setDependencies(prev => prev.map(dep =>
        dep.name === 'yt-dlp'
          ? ytDlpInstalled
            ? { ...dep, status: 'found', version: result.version }
            : { ...dep, status: 'missing' }
          : dep
      ));
    } catch (err) {
      console.error('Failed to check yt-dlp:', err);
      setDependencies(prev => prev.map(dep =>
        dep.name === 'yt-dlp' ? { ...dep, status: 'missing' } : dep
      ));
    }
    // ffmpeg
    try {
      const result = await CheckFfmpeg();
      ffmpegInstalled = !!result.installed;
      setDependencies(prev => prev.map(dep =>
        dep.name === 'ffmpeg'
          ? ffmpegInstalled
            ? { ...dep, status: 'found', version: result.version }
            : { ...dep, status: 'missing' }
          : dep
      ));
    } catch (err) {
      console.error('Failed to check ffmpeg:', err);
      setDependencies(prev => prev.map(dep =>
        dep.name === 'ffmpeg' ? { ...dep, status: 'missing' } : dep
      ));
    }
    // If both are installed, close the dialog after a short delay
    if (ytDlpInstalled && ffmpegInstalled) {
      setTimeout(() => onClose(), 500);
    }
  };

  const handleDownload = async (depName: string) => {
    setDependencies(prev => prev.map(dep =>
      dep.name === depName ? { ...dep, status: 'downloading', progress: 0 } : dep
    ));

    try {
      if (depName === 'yt-dlp') {
        await DownloadYtDlp();
        const result = await CheckYtDlp();
        if (result.installed) {
          setDependencies(prev => prev.map(dep =>
            dep.name === depName ? { ...dep, status: 'found', version: result.version } : dep
          ));
        } else {
          throw new Error('Download completed but yt-dlp not found');
        }
      } else if (depName === 'ffmpeg') {
        await DownloadFfmpeg();
        const result = await CheckFfmpeg();
        if (result.installed) {
          setDependencies(prev => prev.map(dep =>
            dep.name === depName ? { ...dep, status: 'found', version: result.version } : dep
          ));
        } else {
          throw new Error('Download completed but ffmpeg not found');
        }
      }
      await new Promise(resolve => setTimeout(resolve, 500));
      // Only close if all dependencies are found
      const allFound = dependencies.every(dep => dep.status === 'found' || (dep.name === depName && true));
      if (allFound) onClose();
    } catch (err: any) {
      console.error(`Failed to download ${depName}:`, err);
      setDependencies(prev => prev.map(dep =>
        dep.name === depName ? { ...dep, status: 'missing' } : dep
      ));
    }
  };

  const getStatusIcon = (status: Dependency['status']) => {
    switch (status) {
      case 'checking':
        return <Loader2 className="size-4 text-blue-400 animate-spin" />;
      case 'found':
        return <CheckCircle2 className="size-4 text-green-400" />;
      case 'downloading':
        return <Loader2 className="size-4 text-blue-400 animate-spin" />;
      case 'missing':
        return <XCircle className="size-4 text-red-400" />;
    }
  };

  const getStatusText = (dep: Dependency) => {
    switch (dep.status) {
      case 'checking':
        return 'Checking...';
      case 'found':
        return dep.version ? `Found (${dep.version})` : 'Found';
      case 'downloading':
        return `${dep.progress || 0}%`;
      case 'missing':
        return 'Not found';
    }
  };

  // Only allow closing when all dependencies are found
  const canClose = dependencies.every(dep => dep.status === 'found');

  return (
    <Dialog open={true} onOpenChange={(open) => !open && canClose && onClose()}>
      <DialogContent
        className="max-w-md bg-[#141414] border border-[#262626] text-gray-100"
        hideCloseButton={!canClose}
        onEscapeKeyDown={(e) => { if (!canClose) e.preventDefault(); }}
        onPointerDownOutside={(e) => { if (!canClose) e.preventDefault(); }}
        onInteractOutside={(e) => { if (!canClose) e.preventDefault(); }}
      >
        <DialogHeader>
          <DialogTitle className="text-gray-100">Checking Dependencies</DialogTitle>
        </DialogHeader>

        <div className="space-y-3 py-4">
          {dependencies.map(dep => (
            <div key={dep.name} className="flex items-center gap-3 p-3 bg-[#0a0a0a] border border-[#262626] rounded-lg">
              {getStatusIcon(dep.status)}
              <span className="text-gray-200">{dep.name}</span>
              <span className="ml-auto text-sm text-gray-400">
                {getStatusText(dep)}
              </span>
              {dep.status === 'missing' && (
                <Button
                  size="sm"
                  onClick={() => handleDownload(dep.name)}
                  className="ml-2 !bg-white hover:!bg-zinc-200 !text-black h-7 px-3 text-xs"
                >
                  <Download className="size-2 mr-1" />
                  Download
                </Button>
              )}
              {dep.status === 'downloading' && (
                <div className="ml-2 w-16 bg-[#262626] rounded-full h-2 overflow-hidden">
                  <div
                    className="bg-blue-400 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${dep.progress || 0}%` }}
                  />
                </div>
              )}
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
