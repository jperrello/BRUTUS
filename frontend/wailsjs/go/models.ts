export namespace main {

	export class ChatMessage {
	    role: string;
	    content: string;

	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	    }
	}
	export class AgentSession {
	    id: string;
	    model: string;
	    status: string;
	    cost: number;
	    messages: ChatMessage[];
	    serviceName: string;
	    serviceHost: string;
	    connected: boolean;

	    static createFrom(source: any = {}) {
	        return new AgentSession(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.cost = source["cost"];
	        this.messages = this.convertValues(source["messages"], ChatMessage);
	        this.serviceName = source["serviceName"];
	        this.serviceHost = source["serviceHost"];
	        this.connected = source["connected"];
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

	export class CoordinationStatus {
	    agent_id: string;
	    status: string;
	    current_task: string;
	    last_action: string;
	    is_remote: boolean;

	    static createFrom(source: any = {}) {
	        return new CoordinationStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.agent_id = source["agent_id"];
	        this.status = source["status"];
	        this.current_task = source["current_task"];
	        this.last_action = source["last_action"];
	        this.is_remote = source["is_remote"];
	    }
	}

}
