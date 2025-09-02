package web3ext

import "maps"

var ToolModules = map[string]string{
	"tool":        ToolJs,
	"maintenance": MaintenanceJs,
}

const ToolJs = `
web3._extend({
	property: 'tool',
	methods: [
		new web3._extend.Method({
			name: 'verifyPeerNodeIDAttestation',
			call: 'tool_verifyPeerNodeIDAttestation',
			params: 1,
			inputFormatter: [null],
		}),
		new web3._extend.Method({
			name: 'getAttestedPeers',
			call: 'tool_getAttestedPeers'
		}),
	],
	properties: []
});
`

const MaintenanceJs = `
web3._extend({
	property: 'maintenance',
	methods: [
		new web3._extend.Method({
			name: 'stopAPIFeed',
			call: 'maintenance_stopAPIFeed'
		}),
		new web3._extend.Method({
			name: 'startAPIFeed',
			call: 'maintenance_startAPIFeed'
		}),
	],
	properties: []
});
`

func init() {
	maps.Copy(Modules, ToolModules)
}
