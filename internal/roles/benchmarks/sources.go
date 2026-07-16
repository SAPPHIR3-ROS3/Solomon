//go:build automatic_role_scores

package benchmarks

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/roles"

type sourceField struct {
	keys   []string
	higher bool
}

var characteristicSources = map[string][]sourceField{
	"world_knowledge": {
		{keys: []string{"aa_omniscience_accuracy", "omniscience_accuracy"}, higher: true},
		{keys: []string{"aa_omniscience_non_hallucination_rate", "omniscience_non_hallucination_rate"}, higher: true},
		{keys: []string{"livebench_language"}, higher: true},
		{keys: []string{"artificial_analysis_intelligence_index"}, higher: true},
	},
	"reasoning": {
		{keys: []string{"critpt"}, higher: true},
		{keys: []string{"aa_lcr"}, higher: true},
		{keys: []string{"livebench_reasoning"}, higher: true},
		{keys: []string{"artificial_analysis_intelligence_index"}, higher: true},
	},
	"instruction_following": {
		{keys: []string{"ifbench"}, higher: true},
		{keys: []string{"livebench_instruction_following"}, higher: true},
	},
	"science_and_math": {
		{keys: []string{"scicode"}, higher: true},
		{keys: []string{"livecodebench"}, higher: true},
		{keys: []string{"livebench_mathematics"}, higher: true},
		{keys: []string{"artificial_analysis_math_index"}, higher: true},
	},
	"cost": {
		{keys: []string{"list_price_usd_per_1m_tokens"}, higher: false},
	},
	"real_cost": {
		{keys: []string{"intelligence_index_total_cost_usd"}, higher: false},
		{keys: []string{"coding_agent_total_cost_usd"}, higher: false},
	},
	"speed": {
		{keys: []string{"median_output_tokens_per_second"}, higher: true},
	},
	"long_horizon": {
		{keys: []string{"terminal_bench_v2", "terminal_bench_v2_1"}, higher: true},
		{keys: []string{"gdpval_aa", "gdpval_aa_v2"}, higher: true},
		{keys: []string{"artificial_analysis_coding_index"}, higher: true},
	},
	"agentic_capabilities": {
		{keys: []string{"terminal_bench_v2", "terminal_bench_v2_1"}, higher: true},
		{keys: []string{"tau2_bench", "tau3_banking"}, higher: true},
		{keys: []string{"artificial_analysis_coding_index"}, higher: true},
		{keys: []string{"artificial_analysis_agentic_index"}, higher: true},
	},
	"taste": {
		{keys: []string{"gdpval_aa", "gdpval_aa_v2"}, higher: true},
		{keys: []string{"lmarena_webdev"}, higher: true},
		{keys: []string{"lmarena_text_style_control"}, higher: true},
	},
}

func allCharacteristics() []string {
	return roles.AllCharacteristics
}
