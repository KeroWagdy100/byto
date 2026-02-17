import { useState, useEffect } from 'react';
import { FolderOpen } from 'lucide-react';
import { Button } from './ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from './ui/dialog';
import { SelectDownloadFolderWithDefault, GetMediaDefaults, UpdateMediaDefaults, SaveMediaDefaults, AddToQueue } from '../../wailsjs/go/main/App';

// Map backend quality (number) to frontend string
const qualityFromBackend: Record<number, string> = {
    0: '360p',
    1: '480p',
    2: '720p',
    3: '1080p',
    4: '1440p',
    5: '2160p',
};

interface AddMediaDialogProps {
    url: string;
    open: boolean;
    onClose: () => void;
    onSuccess: (id: string, quality: string, path: string) => void;
}

export function AddMediaDialog({ url, open, onClose, onSuccess }: AddMediaDialogProps) {
    const [quality, setQuality] = useState('1080p');
    const [downloadPath, setDownloadPath] = useState('');
    const [onlyAudio, setOnlyAudio] = useState(false);
    const [isLoading, setIsLoading] = useState(true);

    // Load media defaults when dialog opens
    useEffect(() => {
        if (open) {
            loadDefaults();
        }
    }, [open]);

    const loadDefaults = async () => {
        setIsLoading(true);
        try {
            const defaults = await GetMediaDefaults();
            if (defaults) {
                setQuality(qualityFromBackend[defaults.quality] || '1080p');
                setDownloadPath(defaults.download_path || '');
                setOnlyAudio(defaults.only_audio || false);
            }
        } catch (error) {
            console.error('Error loading media defaults:', error);
        } finally {
            setIsLoading(false);
        }
    };

    const handleSelectFolder = async () => {
        try {
            const path = await SelectDownloadFolderWithDefault(downloadPath);
            if (path) {
                setDownloadPath(path);
            }
        } catch (error) {
            console.error('Error selecting folder:', error);
        }
    };

    const handleAdd = async () => {
        try {
            // Add to queue with selected quality and path
            const id = await AddToQueue(url, quality, downloadPath, onlyAudio);

            // Save these settings as defaults for next time
            await UpdateMediaDefaults(quality, downloadPath, onlyAudio);
            await SaveMediaDefaults();

            onSuccess(id, quality, downloadPath);
            onClose();
        } catch (error) {
            console.error('Error adding to queue:', error);
        }
    };

    const handleCancel = () => {
        onClose();
    };

    return (
        <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
            <DialogContent className="max-w-lg bg-[#141414] border border-[#262626] text-gray-100">
                <DialogHeader>
                    <DialogTitle className="text-gray-100">Add Download</DialogTitle>
                    <DialogDescription className="text-gray-400">
                        Configure quality and download location for this media
                    </DialogDescription>
                </DialogHeader>

                {isLoading ? (
                    <div className="py-8 text-center text-gray-400">Loading...</div>
                ) : (
                    <div className="space-y-4 py-4">
                        {/* Quality Selection */}
                        <div>
                            <label className="text-gray-300 text-sm">Video Quality</label>
                            <p className="text-gray-500 text-xs mb-2">Select preferred quality for this download</p>
                            <select
                                value={quality}
                                onChange={(e) => setQuality(e.target.value)}
                                disabled={onlyAudio}
                                className={`w-full px-3 py-2 bg-[#1f1f1f] border border-[#262626] rounded text-sm text-gray-100 ${onlyAudio ? 'opacity-50 cursor-not-allowed' : ''}`}
                            >
                                <option value="360p">360p</option>
                                <option value="480p">480p</option>
                                <option value="720p">720p (HD)</option>
                                <option value="1080p">1080p (Full HD)</option>
                                <option value="1440p">1440p (2K)</option>
                                <option value="2160p">2160p (4K)</option>
                            </select>
                        </div>

                        {/* Audio Only Checkbox */}
                        <div className="flex items-center gap-2">
                            <input
                                type="checkbox"
                                id="onlyAudio"
                                checked={onlyAudio}
                                onChange={(e) => setOnlyAudio(e.target.checked)}
                                className="w-4 h-4 rounded border-[#262626] bg-[#1f1f1f] accent-blue-600 cursor-pointer"
                            />
                            <label htmlFor="onlyAudio" className="text-gray-300 text-sm cursor-pointer select-none">
                                Download as audio
                            </label>
                        </div>

                        {/* Download Path */}
                        <div>
                            <label className="text-gray-300 text-sm">Download Location</label>
                            <p className="text-gray-500 text-xs mb-2">Where this file will be saved</p>
                            <div className="flex gap-2">
                                <input
                                    type="text"
                                    value={downloadPath}
                                    onChange={(e) => setDownloadPath(e.target.value)}
                                    className="flex-1 px-3 py-2 bg-[#1f1f1f] border border-[#262626] rounded text-sm text-gray-100"
                                />
                                <Button
                                    size="sm"
                                    variant="outline"
                                    className="border-[#262626] hover:bg-[#1f1f1f]"
                                    onClick={handleSelectFolder}
                                >
                                    <FolderOpen className="size-4" />
                                </Button>
                            </div>
                        </div>
                    </div>
                )}

                <DialogFooter className="flex justify-end gap-2 pt-4 border-t border-[#262626]">
                    <Button variant="outline" onClick={handleCancel} className="border-[#262626] hover:bg-[#1f1f1f]">
                        Cancel
                    </Button>
                    <Button onClick={handleAdd} className="bg-blue-600 hover:bg-blue-700" disabled={isLoading || !downloadPath}>
                        Add to Queue
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
