export namespace main {

	export class desktopCatalogItem {
	    badge: string;
	    detail: string;
	    id: string;
	    scores: desktopSubagentScore[];
	    title: string;

	    static createFrom(source: any = {}) {
	        return new desktopCatalogItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.badge = source["badge"];
	        this.detail = source["detail"];
	        this.id = source["id"];
	        this.scores = this.convertValues(source["scores"], desktopSubagentScore);
	        this.title = source["title"];
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
	export class desktopSubagentScore {
	    id: string;
	    label: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new desktopSubagentScore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.value = source["value"];
	    }
	}
	export class desktopScorePatch {
	    id: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new desktopScorePatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.value = source["value"];
	    }
	}
	export class desktopChat {
	    id: string;
	    lastMessageAt: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopChat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.lastMessageAt = source["lastMessageAt"];
	        this.title = source["title"];
	    }
	}
	export class desktopProject {
	    chats: desktopChat[];
	    id: string;
	    name: string;
	    path: string;
	    chatCount: number;
	
	    static createFrom(source: any = {}) {
	        return new desktopProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chats = this.convertValues(source["chats"], desktopChat);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.chatCount = source["chatCount"];
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
	export class desktopRule {
	    id: number;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.text = source["text"];
	    }
	}
	export class desktopCharacteristic {
	    id: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopCharacteristic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	    }
	}
	export class desktopRolesTable {
	    catalog: desktopCharacteristic[];
	    characteristics: string[];
	    max: number;
	
	    static createFrom(source: any = {}) {
	        return new desktopRolesTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.catalog = this.convertValues(source["catalog"], desktopCharacteristic);
	        this.characteristics = source["characteristics"];
	        this.max = source["max"];
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
	export class desktopSidebarData {
	    projects: desktopProject[];
	    reasoningEffort: string;
	    userName: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopSidebarData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projects = this.convertValues(source["projects"], desktopProject);
	        this.reasoningEffort = source["reasoningEffort"];
	        this.userName = source["userName"];
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
	export class desktopModelChoice {
	    model: string;
	    provider: string;

	    static createFrom(source: any = {}) {
	        return new desktopModelChoice(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.provider = source["provider"];
	    }
	}
	export class desktopProviderCatalog {
	    complete: boolean;
	    models: string[];
	    provider: string;

	    static createFrom(source: any = {}) {
	        return new desktopProviderCatalog(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.complete = source["complete"];
	        this.models = source["models"];
	        this.provider = source["provider"];
	    }
	}
	export class desktopModelCatalog {
	    current: desktopModelChoice;
	    providers: desktopProviderCatalog[];
	    recent: desktopModelChoice[];

	    static createFrom(source: any = {}) {
	        return new desktopModelCatalog(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = this.convertValues(source["current"], desktopModelChoice);
	        this.providers = this.convertValues(source["providers"], desktopProviderCatalog);
	        this.recent = this.convertValues(source["recent"], desktopModelChoice);
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

}
