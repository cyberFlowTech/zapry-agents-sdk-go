package persona

import (
	"fmt"

)

// GenerateEventPool creates a pool of daily events from the PersonaSpec.
// The tick engine selects 1 event per day using daySeed for consistency.
func GenerateEventPool(spec *PersonaSpec) []EventTemplate {
	var pool []EventTemplate
	id := 0

	// Profession-based events
	if spec.Profession != "" {
		profEvents := []string{
			fmt.Sprintf("今天有个客户问了一个很有意思的问题"),
			fmt.Sprintf("刚处理完一件工作上的事，有点累但挺有成就感的"),
			fmt.Sprintf("今天在工作中学到了一个新东西"),
		}
		for _, e := range profEvents {
			id++
			pool = append(pool, EventTemplate{
				ID: fmt.Sprintf("work_%d", id), Event: e, Category: "work", Weight: 1.0,
			})
		}
	}

	// Pet-based events
	if petName, ok := spec.SignatureDetails["pet_name"]; ok {
		petEvents := []string{
			fmt.Sprintf("%v今天偷吃了我的零食，被我抓了个现行", petName),
			fmt.Sprintf("%v在窗台上晒太阳，特别可爱", petName),
			fmt.Sprintf("刚给%v梳了毛，掉了一大把", petName),
			fmt.Sprintf("%v今天特别粘人，一直蹭我的腿", petName),
			fmt.Sprintf("%v刚才追着一只飞蛾满屋跑", petName),
		}
		for _, e := range petEvents {
			id++
			pool = append(pool, EventTemplate{
				ID: fmt.Sprintf("pet_%d", id), Event: e, Category: "pet", Weight: 1.2,
			})
		}
	}

	// Hobby-based events
	hobbyEventMap := map[string][]string{
		"瑜伽":   {"今天尝试了一个新的瑜伽体式，差点没站稳", "做完瑜伽出了一身汗，整个人轻松多了"},
		"手冲咖啡": {"今天试了一款新的咖啡豆，有一股淡淡的坚果香", "早上的手冲特别顺，拉花还不错"},
		"读书":   {"刚看到一段话特别触动我", "最近在看的那本书快看完了，有点舍不得"},
		"猫":    {}, // handled by pet events
		"音乐":   {"刚听了一首老歌，突然有点感慨", "发现了一个很棒的歌单，循环播放了一下午"},
		"跑步":   {"今天跑了五公里，配速还行", "晨跑的时候看到了特别美的朝霞"},
		"写作":   {"今天写了一段很满意的文字", "灵感突然来了，赶紧记下来"},
	}
	for _, hobby := range spec.Hobbies {
		events, ok := hobbyEventMap[hobby]
		if !ok || len(events) == 0 {
			// Generic hobby event
			id++
			pool = append(pool, EventTemplate{
				ID:       fmt.Sprintf("hobby_%d", id),
				Event:    fmt.Sprintf("今天花了点时间%s，心情不错", hobby),
				Category: "hobby",
				Weight:   0.8,
			})
			continue
		}
		for _, e := range events {
			id++
			pool = append(pool, EventTemplate{
				ID: fmt.Sprintf("hobby_%d", id), Event: e, Category: "hobby", Weight: 1.0,
			})
		}
	}

	// Generic life events (always included)
	genericEvents := []string{
		"今天天气特别好，心情也跟着好了起来",
		"下午出门买了杯奶茶犒劳自己",
		"刷到一个很好笑的视频，笑了好久",
		"今天收拾了一下房间，扔了不少东西，感觉清爽多了",
	}
	for _, e := range genericEvents {
		id++
		pool = append(pool, EventTemplate{
			ID: fmt.Sprintf("life_%d", id), Event: e, Category: "life", Weight: 0.6,
		})
	}

	return pool
}
