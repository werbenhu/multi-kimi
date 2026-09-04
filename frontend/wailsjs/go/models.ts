export namespace main {
	
	export class UsageWindow {
	    kind: string;
	    label: string;
	    usedPercent: number;
	    remainingPercent: number;
	    resetsAt?: string;
	    resetsIn?: string;
	
	    static createFrom(source: any = {}) {
	        return new UsageWindow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.usedPercent = source["usedPercent"];
	        this.remainingPercent = source["remainingPercent"];
	        this.resetsAt = source["resetsAt"];
	        this.resetsIn = source["resetsIn"];
	    }
	}
	export class KimiUsage {
	    status: string;
	    plan?: string;
	    session?: UsageWindow;
	    weekly?: UsageWindow;
	    error?: string;
	    fetchedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new KimiUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.plan = source["plan"];
	        this.session = this.convertValues(source["session"], UsageWindow);
	        this.weekly = this.convertValues(source["weekly"], UsageWindow);
	        this.error = source["error"];
	        this.fetchedAt = source["fetchedAt"];
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
	export class ProfileView {
	    name: string;
	    createdAt: string;
	    updatedAt: string;
	    hasCredential: boolean;
	    isActive: boolean;
	    freshness: string;
	    freshnessNote: string;
	    usage?: KimiUsage;
	
	    static createFrom(source: any = {}) {
	        return new ProfileView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.hasCredential = source["hasCredential"];
	        this.isActive = source["isActive"];
	        this.freshness = source["freshness"];
	        this.freshnessNote = source["freshnessNote"];
	        this.usage = this.convertValues(source["usage"], KimiUsage);
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
	export class CliView {
	    id: string;
	    name: string;
	    livePath: string;
	    livePathShort: string;
	    liveExists: boolean;
	    activeProfile: string;
	    profiles: ProfileView[];
	    envWarnings: string[];
	    liveUsage?: KimiUsage;
	
	    static createFrom(source: any = {}) {
	        return new CliView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.livePath = source["livePath"];
	        this.livePathShort = source["livePathShort"];
	        this.liveExists = source["liveExists"];
	        this.activeProfile = source["activeProfile"];
	        this.profiles = this.convertValues(source["profiles"], ProfileView);
	        this.envWarnings = source["envWarnings"];
	        this.liveUsage = this.convertValues(source["liveUsage"], KimiUsage);
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
	
	
	export class SwitchResult {
	    from: string;
	    to: string;
	    noOp: boolean;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new SwitchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.noOp = source["noOp"];
	        this.warnings = source["warnings"];
	    }
	}

}

