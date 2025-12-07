import { useState, useEffect } from 'react';
import { Settings, Download, FolderOpen, Plus, Play, Pause, RefreshCw, Heart } from 'lucide-react';
import { Button } from './components/ui/button';
import { Input } from './components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './components/ui/select';
import { DownloadItem } from './components/DownloadItem';
import { SettingsPanel } from './components/SettingsPanel';
import { SupportPanel } from './components/SupportPanel';
import { AddToQueue, GetQueue, RemoveFromQueue, StartDownloads, PauseDownloads, StartSingleDownload, PauseSingleDownload, GetSettings, UpdateSettings, SelectDownloadFolder, GetDefaultDownloadPath, ShowInFolder } from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import { domain } from '../wailsjs/go/models';
import bytoLogo from 'figma:asset/e1c6c4d1df3cefc4435d7cc603c42e22f058f10f.png';

// Map backend status (number) to frontend status (string)
const statusMap: Record<number, 'pending' | 'downloading' | 'paused' | 'completed' | 'error'> = {
    0: 'pending',     // Pending
    1: 'downloading', // InProgress
    2: 'completed',   // Completed
    3: 'error',       // Failed
    4: 'paused',      // Paused
};

// Map frontend quality string to backend
const qualityToBackend: Record<string, string> = {
    '2160p': '2160p',
    '1440p': '1440p',
    '1080p': '1080p',
    '720p': '720p',
    '480p': '480p',
    '360p': '360p',
    'best': '1080p',
};

// Map backend quality (number) to frontend string
const qualityFromBackend: Record<number, string> = {
    0: '360p',
    1: '480p',
    2: '720p',
    3: '1080p',
    4: '1440p',
    5: '2160p',
};

interface DownloadVideo {
    id: string;
    url: string;
    fileName: string;
    filePath: string;
    progress: number;
    fileSize: string;
    status: 'pending' | 'downloading' | 'paused' | 'completed' | 'error';
    logs: string[];
}

// Helper to format bytes
function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Convert backend Media to frontend DownloadVideo
function mediaToDownloadVideo(media: domain.Media): DownloadVideo {
    const downloaded = media.progress?.downloaded_bytes || 0;
    const total = media.total_bytes || 0;
    const fileSize = total > 0 
        ? `${formatBytes(downloaded)} / ${formatBytes(total)}`
        : downloaded > 0 ? formatBytes(downloaded) : '--';
    
    return {
        id: media.id,
        url: media.url,
        fileName: media.title || 'Detecting...',
        filePath: media.file_path || '',
        progress: media.progress?.percentage || 0,
        fileSize,
        status: statusMap[media.status] || 'pending',
        logs: media.progress?.logs || [],
    };
}

export default function App() {
    const [urlInput, setUrlInput] = useState('');
    const [downloads, setDownloads] = useState<DownloadVideo[]>([]);
    const [downloadPath, setDownloadPath] = useState('');
    const [quality, setQuality] = useState('1080p');
    const [parallelDownloads, setParallelDownloads] = useState('3');
    const [showSettings, setShowSettings] = useState(false);
    const [showSupport, setShowSupport] = useState(false);
    const [isLoading, setIsLoading] = useState(true);

    // Load initial data from backend
    useEffect(() => {
        const initializeApp = async () => {
            try {
                // Get default download path
                const defaultPath = await GetDefaultDownloadPath();
                setDownloadPath(defaultPath);

                // Get current settings
                const settings = await GetSettings();
                if (settings) {
                    setQuality(qualityFromBackend[settings.quality] || '1080p');
                    setParallelDownloads(settings.parallel_downloads?.toString() || '3');
                    if (settings.download_path) {
                        setDownloadPath(settings.download_path);
                    }
                }

                // Get current queue
                const queue = await GetQueue();
                if (queue && queue.length > 0) {
                    setDownloads(queue.map(mediaToDownloadVideo));
                }
            } catch (error) {
                console.error('Error initializing app:', error);
            } finally {
                setIsLoading(false);
            }
        };

        initializeApp();
    }, []);

    // Set up event listeners for download progress and status updates
    useEffect(() => {
        const unsubProgress = EventsOn('download_progress', (data: { 
            id: string; 
            title?: string;
            total_bytes?: number;
            progress: domain.DownloadProgress 
        }) => {
            setDownloads(prev => prev.map(d => {
                if (d.id === data.id) {
                    const downloaded = data.progress.downloaded_bytes || 0;
                    const total = data.total_bytes || 0;
                    const fileSize = total > 0 
                        ? `${formatBytes(downloaded)} / ${formatBytes(total)}`
                        : downloaded > 0 ? formatBytes(downloaded) : d.fileSize;
                    
                    return {
                        ...d,
                        fileName: data.title && data.title !== 'NA' && data.title !== '' ? data.title : d.fileName,
                        progress: data.progress.percentage || 0,
                        logs: data.progress.logs || d.logs,
                        fileSize,
                    };
                }
                return d;
            }));
        });

        const unsubStatus = EventsOn('download_status', (data: { id: string; status: number }) => {
            setDownloads(prev => prev.map(d => {
                if (d.id === data.id) {
                    return {
                        ...d,
                        status: statusMap[data.status] || 'pending',
                    };
                }
                return d;
            }));
        });

        const unsubTitle = EventsOn('download_title', (data: { id: string; title: string }) => {
            setDownloads(prev => prev.map(d => {
                if (d.id === data.id) {
                    return {
                        ...d,
                        fileName: data.title || d.fileName,
                    };
                }
                return d;
            }));
        });

        return () => {
            EventsOff('download_progress');
            EventsOff('download_status');
            EventsOff('download_title');
        };
    }, []);

    // Sync settings to backend when they change
    useEffect(() => {
        if (!isLoading) {
            const parallel = parallelDownloads === 'unlimited' ? 100 : parseInt(parallelDownloads, 10);
            UpdateSettings(qualityToBackend[quality] || quality, parallel, downloadPath);
        }
    }, [quality, parallelDownloads, downloadPath, isLoading]);

    const handleAddUrl = async () => {
        if (!urlInput.trim()) return;
        
        try {
            // Open folder picker with default path pre-selected
            const selectedPath = await SelectDownloadFolder();
            if (!selectedPath) {
                // User cancelled folder selection
                return;
            }
            
            const id = await AddToQueue(urlInput.trim(), selectedPath);
            const newDownload: DownloadVideo = {
                id,
                url: urlInput,
                fileName: 'Detecting...',
                filePath: selectedPath,
                progress: 0,
                fileSize: '--',
                status: 'pending',
                logs: []
            };
            setDownloads([...downloads, newDownload]);
            setUrlInput('');
        } catch (error) {
            console.error('Error adding URL to queue:', error);
        }
    };

    const handleToggleAll = async () => {
        if (activeDownloads > 0) {
            // Pause all downloading items
            try {
                await PauseDownloads();
                // Status updates will come from backend events (download_status)
            } catch (error) {
                console.error('Error pausing downloads:', error);
            }
        } else {
            // Start downloads
            try {
                await StartDownloads();
                // Status updates will come from backend events (download_status)
            } catch (error) {
                console.error('Error starting downloads:', error);
            }
        }
    };

    const handleDownloadAction = async (id: string, action: 'start' | 'pause' | 'resume' | 'delete') => {
        try {
            if (action === 'start' || action === 'resume') {
                await StartSingleDownload(id);
                // Status updates will come from backend events
            } else if (action === 'pause') {
                await PauseSingleDownload(id);
                // Status updates will come from backend events
            }
        } catch (error) {
            console.error(`Error performing ${action} on download:`, error);
        }
    };

    const handleRemoveDownload = async (id: string) => {
        try {
            await RemoveFromQueue(id);
            setDownloads(downloads.filter(d => d.id !== id));
        } catch (error) {
            console.error('Error removing download:', error);
            // Still remove from UI even if backend fails
            setDownloads(downloads.filter(d => d.id !== id));
        }
    };

    const handleShowInFolder = async (id: string) => {
        const download = downloads.find(d => d.id === id);
        if (download && download.filePath) {
            console.log(`Opening folder for: ${download.fileName} at ${download.filePath}`);
            try {
                // Pass the download's file path to open in explorer
                await ShowInFolder(download.filePath);
            } catch (error) {
                console.error('Error showing in folder:', error);
            }
        }
    };

    const handleSelectFolder = async () => {
        try {
            const path = await SelectDownloadFolder();
            if (path) {
                setDownloadPath(path);
            }
        } catch (error) {
            console.error('Error selecting folder:', error);
        }
    };

    const activeDownloads = downloads.filter(d => d.status === 'downloading').length;

    return (
        <div className="min-h-screen bg-[#0a0a0a] flex flex-col">
            {/* Title Bar */}
            <div className="bg-[#141414] border-b border-[#262626] px-4 py-3 flex items-center justify-between">
                <div className="flex items-center gap-3">
                    <img src={bytoLogo} alt="Byto" className="size-6" />
                    <h2 className="text-gray-100">byto</h2>
                </div>
                <div className="flex items-center gap-2">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowSupport(true)}
                        className="border-[#262626] hover:bg-[#1f1f1f]"
                    >
                        <Heart className="size-4" />
                        Support
                    </Button>
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowSettings(!showSettings)}
                        className="border-[#262626] hover:bg-[#1f1f1f]"
                    >
                        <Settings className="size-4" />
                        Settings
                    </Button>
                    <Button variant="outline" size="sm" className="border-[#262626] hover:bg-[#1f1f1f]">
                        <RefreshCw className="size-4" />
                        Update
                    </Button>
                </div>
            </div>

            {/* Settings Panel */}
            {showSettings && (
                <SettingsPanel
                    downloadPath={downloadPath}
                    setDownloadPath={setDownloadPath}
                    quality={quality}
                    setQuality={setQuality}
                    parallelDownloads={parallelDownloads}
                    setParallelDownloads={setParallelDownloads}
                    onClose={() => setShowSettings(false)}
                />
            )}

            {/* Support Panel */}
            {showSupport && (
                <SupportPanel onClose={() => setShowSupport(false)} />
            )}

            {/* Main Content */}
            <div className="flex-1 overflow-auto p-6">
                <div className="max-w-7xl mx-auto space-y-6">
                    {/* Add URL Section */}
                    <div className="bg-[#141414] rounded-lg border border-[#262626] p-4">
                        <div className="flex gap-2">
                            <Input
                                placeholder="Paste video, audio, playlist URL here"
                                value={urlInput}
                                onChange={(e) => setUrlInput(e.target.value)}
                                onKeyDown={(e) => e.key === 'Enter' && handleAddUrl()}
                                className="flex-1"
                            />
                            <Button onClick={handleAddUrl}>
                                <Plus className="size-4" />
                                Add URL
                            </Button>
                        </div>
                    </div>

                    {/* Control Bar */}
                    <div className="bg-[#141414] rounded-lg border border-[#262626] p-4">
                        <div className="flex items-center justify-between">
                            <div className="text-gray-400">
                                <span>{downloads.length} total</span>
                                <span className="mx-2">•</span>
                                <span>{activeDownloads} downloading</span>
                                <span className="mx-2">•</span>
                                <span>{downloads.filter(d => d.status === 'completed').length} completed</span>
                            </div>
                            <div className="flex gap-2">
                                <Button onClick={handleToggleAll}>
                                    {activeDownloads > 0 ? (
                                        <>
                                            <Pause className="size-4" />
                                            Pause
                                        </>
                                    ) : (
                                        <>
                                            <Play className="size-4" />
                                            Start
                                        </>
                                    )}
                                </Button>
                            </div>
                        </div>
                    </div>

                    {/* Downloads List */}
                    <div className="space-y-3">
                        {downloads.length === 0 ? (
                            <div className="bg-[#141414] rounded-lg border border-[#262626] p-12 text-center text-gray-500">
                                <Download className="size-12 mx-auto mb-4 opacity-50" />
                                <p>No downloads yet. Add a URL to get started.</p>
                            </div>
                        ) : (
                            downloads.map(download => (
                                <DownloadItem
                                    key={download.id}
                                    download={download}
                                    onAction={handleDownloadAction}
                                    onRemove={handleRemoveDownload}
                                    onShowInFolder={handleShowInFolder}
                                />
                            ))
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}
