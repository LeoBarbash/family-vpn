export namespace vpn {
	
	export class Status {
	    connected: boolean;
	    interface: string;
	    endpoint: string;
	    message: string;
	    since?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.interface = source["interface"];
	        this.endpoint = source["endpoint"];
	        this.message = source["message"];
	        this.since = source["since"];
	    }
	}

}

