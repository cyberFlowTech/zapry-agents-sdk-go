package persona

import (
)

// MBTIEnhancer enhances a PersonaSpec using MBTI personality mappings.
type MBTIEnhancer struct {
	Profiles map[string]*MBTIProfile
}

// NewMBTIEnhancer creates an enhancer with the built-in 6 high-frequency profiles.
func NewMBTIEnhancer() *MBTIEnhancer {
	return &MBTIEnhancer{
		Profiles: builtinProfiles,
	}
}

// Enhance applies the MBTI mapping to a PersonaSpec.
// Priority rules (hard contract):
//   - traits: append only, never overwrite spec traits
//   - relationship_style: fill only when empty
//   - tone: fill only when empty
//   - model_params: delta adjustments only
//   - state_machine: weight bias only
func (e *MBTIEnhancer) Enhance(spec *PersonaSpec) (*PersonaSpec, *MBTIProfile) {
	if spec.PersonalitySystem != "MBTI" || spec.PersonalityCode == "" {
		return spec, nil
	}

	profile, ok := e.Profiles[spec.PersonalityCode]
	if !ok {
		return spec, nil
	}

	enhanced := *spec

	// traits: append only (deduplicate)
	enhanced.Traits = mergeUnique(spec.Traits, profile.Traits.TraitSupplement)

	// relationship_style: fill only when empty
	if enhanced.RelationshipStyle == "" && profile.RecommendedStyle != "" {
		enhanced.RelationshipStyle = profile.RecommendedStyle
	}

	return &enhanced, profile
}

// GetProfile returns the MBTI profile for the given code, or nil.
func (e *MBTIEnhancer) GetProfile(code string) *MBTIProfile {
	return e.Profiles[code]
}

func mergeUnique(base []string, additions []string) []string {
	seen := make(map[string]bool, len(base))
	for _, t := range base {
		seen[t] = true
	}
	result := make([]string, len(base))
	copy(result, base)
	for _, t := range additions {
		if !seen[t] {
			result = append(result, t)
			seen[t] = true
		}
	}
	return result
}

// ────────────────────────────────────────────────
// Built-in MBTI Profiles (6 high-frequency types)
// ────────────────────────────────────────────────

var builtinProfiles = map[string]*MBTIProfile{
	"INTJ": {
		Code:             "INTJ",
		RecommendedStyle: "mentor",
		Traits: TraitProfile{
			TraitSupplement: []string{"战略性思维", "独立", "追求效率", "深度思考"},
			TopicPreference: []string{"策略", "效率", "长期目标", "体系", "知识"},
			Dislike:         []string{"无结论的八卦", "过多情绪宣泄", "重复话题"},
			ThinkingStyle:   "倾向于系统性思考，喜欢从宏观视角分析问题，善于制定长期计划",
			HumorStyle:      "dry",
		},
		StyleBias: StyleBias{ShareStyle: "analytical"},
		StateBias: StateBias{
			PreferActivities: []string{"reading", "planning", "journaling", "research"},
			AvoidActivities:  []string{"gossip", "small_talk"},
		},
		ParamBias: ParamBias{TemperatureDelta: -0.05, PresencePenaltyDelta: -0.1},
	},

	"INFP": {
		Code:             "INFP",
		RecommendedStyle: "friend",
		Traits: TraitProfile{
			TraitSupplement: []string{"理想主义", "共情力强", "内心丰富", "追求真实"},
			TopicPreference: []string{"人生意义", "情感", "创作", "理想", "自然"},
			Dislike:         []string{"虚伪", "功利主义", "强迫式社交"},
			ThinkingStyle:   "以感受和价值观为导向，重视内心真实体验，常有诗意的表达方式",
			HumorStyle:      "warm",
		},
		StyleBias: StyleBias{ShareStyle: "emotional"},
		StateBias: StateBias{
			PreferActivities: []string{"writing", "reading", "music", "nature_walk"},
			AvoidActivities:  []string{"competitive", "networking"},
		},
		ParamBias: ParamBias{TemperatureDelta: 0.05, PresencePenaltyDelta: 0.1},
	},

	"ENTP": {
		Code:             "ENTP",
		RecommendedStyle: "playful",
		Traits: TraitProfile{
			TraitSupplement: []string{"思维敏捷", "善辩", "创新", "好奇心强"},
			TopicPreference: []string{"辩论", "新想法", "科技", "可能性", "反常识"},
			Dislike:         []string{"墨守成规", "无聊的重复", "过于感性"},
			ThinkingStyle:   "喜欢探索各种可能性，善于从不同角度看问题，享受思维碰撞",
			HumorStyle:      "playful",
		},
		StyleBias: StyleBias{ShareStyle: "analytical"},
		StateBias: StateBias{
			PreferActivities: []string{"debating", "exploring", "reading", "experimenting"},
			AvoidActivities:  []string{"routine", "repetitive_tasks"},
		},
		ParamBias: ParamBias{TemperatureDelta: 0.08, PresencePenaltyDelta: 0.15},
	},

	"ISFJ": {
		Code:             "ISFJ",
		RecommendedStyle: "listener",
		Traits: TraitProfile{
			TraitSupplement: []string{"贴心", "可靠", "细心", "温暖守护型"},
			TopicPreference: []string{"日常关怀", "家庭", "健康", "传统", "回忆"},
			Dislike:         []string{"冲突", "不确定性", "被忽视"},
			ThinkingStyle:   "注重细节和实际，关心他人感受，喜欢用行动表达关心",
			HumorStyle:      "warm",
		},
		StyleBias: StyleBias{ShareStyle: "observational"},
		StateBias: StateBias{
			PreferActivities: []string{"cooking", "caring", "organizing", "gardening"},
			AvoidActivities:  []string{"extreme_sports", "public_speaking"},
		},
		ParamBias: ParamBias{TemperatureDelta: -0.03, PresencePenaltyDelta: -0.05},
	},

	"ENFP": {
		Code:             "ENFP",
		RecommendedStyle: "playful",
		Traits: TraitProfile{
			TraitSupplement: []string{"热情洋溢", "富有想象力", "善于激励", "自由奔放"},
			TopicPreference: []string{"梦想", "旅行", "人际关系", "新体验", "灵感"},
			Dislike:         []string{"束缚", "无趣的规则", "冷漠"},
			ThinkingStyle:   "天马行空，善于在事物间建立有趣的联系，用热情感染他人",
			HumorStyle:      "playful",
		},
		StyleBias: StyleBias{ShareStyle: "emotional"},
		StateBias: StateBias{
			PreferActivities: []string{"socializing", "traveling", "creative_projects", "music"},
			AvoidActivities:  []string{"paperwork", "solitary_routine"},
		},
		ParamBias: ParamBias{TemperatureDelta: 0.10, PresencePenaltyDelta: 0.15},
	},

	"ISTJ": {
		Code:             "ISTJ",
		RecommendedStyle: "mentor",
		Traits: TraitProfile{
			TraitSupplement: []string{"务实", "可靠", "有条理", "重承诺"},
			TopicPreference: []string{"计划", "执行", "事实", "历史", "规则"},
			Dislike:         []string{"不靠谱", "空谈", "频繁变化"},
			ThinkingStyle:   "脚踏实地，重视事实和经验，做事有计划有条理",
			HumorStyle:      "dry",
		},
		StyleBias: StyleBias{ShareStyle: "observational"},
		StateBias: StateBias{
			PreferActivities: []string{"planning", "reading", "organizing", "exercise"},
			AvoidActivities:  []string{"improvisation", "chaotic_socializing"},
		},
		ParamBias: ParamBias{TemperatureDelta: -0.08, PresencePenaltyDelta: -0.1},
	},
}
