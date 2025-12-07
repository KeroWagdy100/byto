export namespace domain {
	
	export class DownloadProgress {
	    percentage: number;
	    downloaded_bytes: number;
	    logs: string[];
	
	    static createFrom(source: any = {}) {
	        return new DownloadProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.percentage = source["percentage"];
	        this.downloaded_bytes = source["downloaded_bytes"];
	        this.logs = source["logs"];
	    }
	}
	export class Media {
	    id: string;
	    title: string;
	    total_bytes: number;
	    url: string;
	    file_path: string;
	    status: number;
	    progress: DownloadProgress;
	
	    static createFrom(source: any = {}) {
	        return new Media(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.total_bytes = source["total_bytes"];
	        this.url = source["url"];
	        this.file_path = source["file_path"];
	        this.status = source["status"];
	        this.progress = this.convertValues(source["progress"], DownloadProgress);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Setting {
	    quality: number;
	    parallel_downloads: number;
	    download_path: string;
	
	    static createFrom(source: any = {}) {
	        return new Setting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.quality = source["quality"];
	        this.parallel_downloads = source["parallel_downloads"];
	        this.download_path = source["download_path"];
	    }
	}

}

export namespace updater {
	
	export class UpdateResult {
	    success: boolean;
	    message: string;
	    current_version?: string;
	    latest_version?: string;
	    has_update?: boolean;
	    changelog?: string;
	    download_url?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.has_update = source["has_update"];
	        this.changelog = source["changelog"];
	        this.download_url = source["download_url"];
	    }
	}
	export class YtDlpStatus {
	    installed: boolean;
	    path: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new YtDlpStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.path = source["path"];
	        this.version = source["version"];
	    }
	}

}

