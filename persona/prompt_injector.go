package persona

import (
	"fmt"
	"time"

)

// BuildPromptInjection generates a natural language state description.
func BuildPromptInjection(state *CurrentState, mood *MoodState, todayEvent string, now time.Time, config *RuntimeConfig) string {
	timeDesc := formatTimeDesc(now)
	activityDesc := activityToDesc(state.Activity, config.SignatureDetails)
	moodDesc := mood.Label

	injection := fmt.Sprintf("【当前状态】现在是%s。%s，%s。", timeDesc, activityDesc, moodDesc)

	if todayEvent != "" {
		injection += fmt.Sprintf("\n【今天的小事】%s", todayEvent)
	}

	return injection
}

func formatTimeDesc(now time.Time) string {
	hour := now.Hour()
	minute := now.Minute()

	var period string
	switch {
	case hour >= 5 && hour < 8:
		period = "早上"
	case hour >= 8 && hour < 12:
		period = "上午"
	case hour >= 12 && hour < 14:
		period = "中午"
	case hour >= 14 && hour < 18:
		period = "下午"
	case hour >= 18 && hour < 22:
		period = "晚上"
	default:
		period = "深夜"
	}

	return fmt.Sprintf("%s %d:%02d", period, hour, minute)
}

func activityToDesc(activity string, details map[string]any) string {
	petName := ""
	if pn, ok := details["pet_name"]; ok {
		petName = fmt.Sprintf("%v", pn)
	}

	// Activity-specific descriptions
	descMap := map[string]string{
		"瑜伽":   "刚做完瑜伽，出了一身汗但整个人轻松多了",
		"手冲咖啡": "正在泡一杯手冲，厨房里弥漫着咖啡香",
		"读书":   "窝在沙发上看书",
		"看书":   "窝在沙发上看书",
		"音乐":   "戴着耳机听歌",
		"听音乐":  "戴着耳机听歌",
		"工作":   "在处理一些工作上的事",
		"午餐":   "刚吃完午饭",
		"午休":   "靠在椅子上眯了一会儿",
		"散步":   "出门散了会儿步",
		"跑步":   "刚跑完步回来",
		"做饭":   "在厨房准备做饭",
		"醒来":   "刚醒来，还有点迷糊",
		"准备出门": "正在收拾准备出门",
		"放松":   "靠在沙发上放空",
		"看手机":  "刷了会儿手机",
		"准备睡觉": "洗完澡准备睡了",
		"忙碌中":  "下午一直在忙",
		"休息":   "在休息",
		"游戏":   "打了会儿游戏",
		"写作":   "在写一些东西",
	}

	desc, ok := descMap[activity]
	if !ok {
		desc = fmt.Sprintf("在%s", activity)
	}

	// Add pet context for evening activities
	if petName != "" {
		switch activity {
		case "读书", "看书":
			desc += fmt.Sprintf("，%s在我腿上打呼噜", petName)
		case "放松", "看手机":
			desc += fmt.Sprintf("，%s窝在旁边", petName)
		case "准备睡觉":
			desc += fmt.Sprintf("，%s已经在床上占好位置了", petName)
		}
	}

	return desc
}
