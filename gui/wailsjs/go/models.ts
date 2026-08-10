export namespace main {
	
	export class desktopAtMentionEntry {
	    isDirectory: boolean;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopAtMentionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isDirectory = source["isDirectory"];
	        this.path = source["path"];
	    }
	}
	export class desktopAtMentionSuggestion {
	    isDirectory: boolean;
	    path: string;
	    tag: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopAtMentionSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isDirectory = source["isDirectory"];
	        this.path = source["path"];
	        this.tag = source["tag"];
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
	export class desktopCatalogItem {
	    badge?: string;
	    detail: string;
	    id: string;
	    scores?: desktopSubagentScore[];
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
	export class desktopConnectProviderRequest {
	    APIKey: string;
	    BaseURL: string;
	    Kind: number;
	    Name: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopConnectProviderRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.APIKey = source["APIKey"];
	        this.BaseURL = source["BaseURL"];
	        this.Kind = source["Kind"];
	        this.Name = source["Name"];
	    }
	}
	export class desktopModelMetadata {
	    context?: number;
	    input?: string[];
	    output?: number;
	
	    static createFrom(source: any = {}) {
	        return new desktopModelMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.input = source["input"];
	        this.output = source["output"];
	    }
	}
	export class desktopProviderCatalog {
	    complete: boolean;
	    disabled?: string[];
	    metadata: Record<string, desktopModelMetadata>;
	    models: string[];
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopProviderCatalog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.complete = source["complete"];
	        this.disabled = source["disabled"];
	        this.metadata = this.convertValues(source["metadata"], desktopModelMetadata, true);
	        this.models = source["models"];
	        this.provider = source["provider"];
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
	
	
	export class desktopModelVisibility {
	    enabled: boolean;
	    model: string;
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopModelVisibility(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.model = source["model"];
	        this.provider = source["provider"];
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
	export class desktopProjectBranches {
	    current: string;
	    branches: string[];
	    isRepo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new desktopProjectBranches(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.branches = source["branches"];
	        this.isRepo = source["isRepo"];
	    }
	}
	export class desktopProjectGitCommit {
	    author: string;
	    authoredAt: string;
	    hash: string;
	    parents: string[];
	    refs: string[];
	    shortHash: string;
	    subject: string;

	    static createFrom(source: any = {}) {
	        return new desktopProjectGitCommit(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.author = source["author"];
	        this.authoredAt = source["authoredAt"];
	        this.hash = source["hash"];
	        this.parents = source["parents"];
	        this.refs = source["refs"];
	        this.shortHash = source["shortHash"];
	        this.subject = source["subject"];
	    }
	}
	export class desktopProjectGitHistory {
	    commits: desktopProjectGitCommit[];
	    current: string;
	    isRepo: boolean;

	    static createFrom(source: any = {}) {
	        return new desktopProjectGitHistory(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commits = this.convertValues(source["commits"], desktopProjectGitCommit);
	        this.current = source["current"];
	        this.isRepo = source["isRepo"];
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
	export class desktopProjectDirectoryEntry {
	    isDirectory: boolean;
	    name: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopProjectDirectoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isDirectory = source["isDirectory"];
	        this.name = source["name"];
	        this.path = source["path"];
	    }
	}
	export class desktopProjectRemovalInfo {
	    dataPath: string;
	    dataSizeBytes: number;
	    projectPath: string;
	    projectSizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new desktopProjectRemovalInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataPath = source["dataPath"];
	        this.dataSizeBytes = source["dataSizeBytes"];
	        this.projectPath = source["projectPath"];
	        this.projectSizeBytes = source["projectSizeBytes"];
	    }
	}
	export class desktopProjectWorktree {
	    path: string;
	    branch: string;
	    bare: boolean;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new desktopProjectWorktree(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.bare = source["bare"];
	        this.current = source["current"];
	    }
	}
	export class desktopProjectWorktrees {
	    worktrees: desktopProjectWorktree[];
	
	    static createFrom(source: any = {}) {
	        return new desktopProjectWorktrees(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.worktrees = this.convertValues(source["worktrees"], desktopProjectWorktree);
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
	export class desktopPromptTemplate {
	    content: string;
	    id: string;
	    modified: boolean;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new desktopPromptTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.id = source["id"];
	        this.modified = source["modified"];
	        this.title = source["title"];
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

}

export namespace research {
	
	export class Finding {
	    url: string;
	    title?: string;
	    summary?: string;
	    evidence?: string;
	    rational?: string;
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.evidence = source["evidence"];
	        this.rational = source["rational"];
	    }
	}
	export class JobStats {
	    duration_secs?: number;
	    rounds?: number;
	    queries?: number;
	    urls?: number;
	    findings?: number;
	    url_read_ok?: number;
	    url_fetch_failed?: number;
	    url_empty_content?: number;
	    url_llm_failed?: number;
	    url_low_quality?: number;
	    url_parse_failed?: number;
	    search_failures?: number;
	    search_engine?: string;
	    model?: string;
	    category?: string;
	    prompt_tokens?: number;
	    response_tokens?: number;
	    total_tokens?: number;
	    estimated_tokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new JobStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.duration_secs = source["duration_secs"];
	        this.rounds = source["rounds"];
	        this.queries = source["queries"];
	        this.urls = source["urls"];
	        this.findings = source["findings"];
	        this.url_read_ok = source["url_read_ok"];
	        this.url_fetch_failed = source["url_fetch_failed"];
	        this.url_empty_content = source["url_empty_content"];
	        this.url_llm_failed = source["url_llm_failed"];
	        this.url_low_quality = source["url_low_quality"];
	        this.url_parse_failed = source["url_parse_failed"];
	        this.search_failures = source["search_failures"];
	        this.search_engine = source["search_engine"];
	        this.model = source["model"];
	        this.category = source["category"];
	        this.prompt_tokens = source["prompt_tokens"];
	        this.response_tokens = source["response_tokens"];
	        this.total_tokens = source["total_tokens"];
	        this.estimated_tokens = source["estimated_tokens"];
	    }
	}
	export class URLAttempt {
	    url: string;
	    status: string;
	    detail?: string;
	    // Go type: time
	    at?: any;
	
	    static createFrom(source: any = {}) {
	        return new URLAttempt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	        this.at = this.convertValues(source["at"], null);
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
	export class JobRecord {
	    id: string;
	    slug: string;
	    title: string;
	    question: string;
	    category?: string;
	    status: string;
	    phase?: string;
	    round?: number;
	    max_rounds?: number;
	    parent_chat_id?: string;
	    project_hex: string;
	    html_path?: string;
	    research_plan?: string;
	    evolving_report?: string;
	    findings?: Finding[];
	    queries_used?: string[];
	    urls_fetched?: string[];
	    url_attempts?: URLAttempt[];
	    stats?: JobStats;
	    error?: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    finished_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new JobRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.question = source["question"];
	        this.category = source["category"];
	        this.status = source["status"];
	        this.phase = source["phase"];
	        this.round = source["round"];
	        this.max_rounds = source["max_rounds"];
	        this.parent_chat_id = source["parent_chat_id"];
	        this.project_hex = source["project_hex"];
	        this.html_path = source["html_path"];
	        this.research_plan = source["research_plan"];
	        this.evolving_report = source["evolving_report"];
	        this.findings = this.convertValues(source["findings"], Finding);
	        this.queries_used = source["queries_used"];
	        this.urls_fetched = source["urls_fetched"];
	        this.url_attempts = this.convertValues(source["url_attempts"], URLAttempt);
	        this.stats = this.convertValues(source["stats"], JobStats);
	        this.error = source["error"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.finished_at = this.convertValues(source["finished_at"], null);
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
