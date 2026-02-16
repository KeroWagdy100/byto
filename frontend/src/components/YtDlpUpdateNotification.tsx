import { useState } from 'react';
import { ArrowUpCircle, X, Loader2 } from 'lucide-react';
import { Button } from './ui/button';
import { UpdateYTDLP } from '../../wailsjs/go/main/App';

interface YtDlpUpdateNotificationProps {
  currentVersion: string;
  latestVersion: string;
  onDismiss: () => void;
}

export function YtDlpUpdateNotification({ currentVersion, latestVersion, onDismiss }: YtDlpUpdateNotificationProps) {
  const [updating, setUpdating] = useState(false);
  const [updateResult, setUpdateResult] = useState<{ success: boolean; message: string } | null>(null);

  const handleUpdate = async () => {
    setUpdating(true);
    try {
      const result = await UpdateYTDLP();
      setUpdateResult({ success: result.success, message: result.message });
      if (result.success) {
        setTimeout(() => onDismiss(), 2000);
      }
    } catch (err: any) {
      setUpdateResult({ success: false, message: err?.message || 'Update failed' });
    } finally {
      setUpdating(false);
    }
  };

  return (
    <div className="fixed bottom-4 right-4 z-50 max-w-sm animate-in slide-in-from-bottom-4 fade-in duration-300">
      <div className="bg-[#141414] border border-[#262626] rounded-lg shadow-lg p-4">
        <div className="flex items-start gap-3">
          <ArrowUpCircle className={`size-5 mt-0.5 shrink-0 ${updateResult ? (updateResult.success ? 'text-green-400' : 'text-red-400') : 'text-blue-400'}`} />
          <div className="flex-1 min-w-0">
            {updateResult ? (
              <>
                <p className={`text-sm ${updateResult.success ? 'text-green-400' : 'text-red-400'}`}>
                  {updateResult.success ? 'yt-dlp updated successfully!' : 'Update failed'}
                </p>
                {!updateResult.success && (
                  <p className="text-xs text-gray-500 mt-1 break-words">
                    {updateResult.message}
                  </p>
                )}
              </>
            ) : (
              <>
                <p className="text-sm text-gray-100 font-medium">yt-dlp update available</p>
                <p className="text-xs text-gray-400 mt-1">
                  {currentVersion} → {latestVersion}
                </p>
                <p className="text-xs" style={{ color: '#eab308', marginTop: '12px' }}>
                  Keeping yt-dlp up to date ensures byto works correctly.
                </p>
                <div className="flex items-center gap-2" style={{ marginTop: '12px' }}>
                  <Button
                    size="sm"
                    onClick={handleUpdate}
                    disabled={updating}
                    className="!bg-white hover:!bg-zinc-200 !text-black h-7 px-3 text-xs"
                  >
                    {updating ? (
                      <>
                        <Loader2 className="size-3 mr-1 animate-spin" />
                        Updating...
                      </>
                    ) : (
                      'Update'
                    )}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={onDismiss}
                    disabled={updating}
                    className="text-gray-400 hover:text-gray-200 h-7 px-3 text-xs"
                  >
                    Later
                  </Button>
                </div>
              </>
            )}
          </div>
          {!updating && (
            <button onClick={onDismiss} className="text-gray-500 hover:text-gray-300 shrink-0">
              <X className="size-4" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
