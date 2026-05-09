package ui

type displayLanguage int

const (
	languageJapanese displayLanguage = iota
	languageEnglish
)

const (
	statusSimpleMonitorRunning = "simple monitor running"
	statusETWDisabled          = "ETW helper disabled in release UI"
	statusETWAlreadyRequested  = "ETW helper is already requested"
	statusETWWaiting           = "waiting for ETW helper events"
	statusETWReceived          = "ETW helper events received"
)

type uiStrings struct {
	refreshButton       string
	startETWButton      string
	stopETWButton       string
	openLogsButton      string
	exportVisibleButton string
	languageLabel       string
	languageOptions     []string

	monitoringStatusTitle string
	monitoringPrefix      string
	privilegePrefix       string
	privilegeChecking     string
	standardUser          string
	administrator         string
	preciseETWRequested   string
	summaryFormat         string
	logPrefix             string

	connectedDevicesTitle string
	timelineTitle         string
	typeLabel             string
	confidenceLabel       string
	searchCue             string
	typeOptions           []string
	confidenceOptions     []string
	deviceColumnTitles    []string
	eventColumnTitles     []string
	emptyDetails          string

	deviceDetailsTitle  string
	deviceMonitoring    string
	deviceName          string
	devicePowerState    string
	deviceManufacturer  string
	deviceLocation      string
	deviceLastSeen      string
	eventDetailsTitle   string
	eventMark           string
	eventTime           string
	eventType           string
	eventConfidence     string
	eventMessage        string
	rawETWProperties    string
	markSuspend         string
	markResume          string
	markError           string
	markPnPArrival      string
	markPnPRemoval      string
	exportDialogTitle   string
	exportFailedTitle   string
	exportCompleteTitle string
	trayShow            string
	trayOpenLogs        string
	trayExit            string
	trayTooltip         string
	notifyResumeTitle   string
	notifySuspendTitle  string
	unknownUSBDevice    string
	monitorOn           string
	monitorOff          string

	statusText map[string]string
}

func languageFromIndex(index int) displayLanguage {
	if index == int(languageEnglish) {
		return languageEnglish
	}
	return languageJapanese
}

func (l displayLanguage) index() int {
	if l == languageEnglish {
		return int(languageEnglish)
	}
	return int(languageJapanese)
}

func stringsFor(language displayLanguage) uiStrings {
	if language == languageEnglish {
		return englishStrings()
	}
	return japaneseStrings()
}

func japaneseStrings() uiStrings {
	return uiStrings{
		refreshButton:       "更新",
		startETWButton:      "ETW開始（実験）",
		stopETWButton:       "ETW停止",
		openLogsButton:      "ログフォルダ",
		exportVisibleButton: "表示ログ出力",
		languageLabel:       "言語",
		languageOptions:     []string{"日本語", "English"},

		monitoringStatusTitle: "監視状態",
		monitoringPrefix:      "監視",
		privilegePrefix:       "権限",
		privilegeChecking:     "確認中",
		standardUser:          "標準ユーザー",
		administrator:         "管理者",
		preciseETWRequested:   "精密ETW要求中",
		summaryFormat:         "USB: %d | 監視: %d | 低電力: %d | Suspend疑い: %d | Resume: %d | 表示: %d",
		logPrefix:             "ログ",

		connectedDevicesTitle: "接続中USB",
		timelineTitle:         "Suspend / Resume タイムライン",
		typeLabel:             "種別",
		confidenceLabel:       "信頼度",
		searchCue:             "デバイス名、VID/PID、Instance ID、メッセージを検索",
		typeOptions:           []string{"すべて", "Suspend疑い", "Resume", "PnP", "エラー"},
		confidenceOptions:     []string{"すべて", "High+Medium", "High only"},
		deviceColumnTitles:    []string{"監視・名前", "VID/PID", "電源", "列挙子", "場所", "最終確認"},
		eventColumnTitles:     []string{"重要", "時刻", "イベント", "信頼度", "ソース", "デバイス", "メッセージ"},
		emptyDetails:          "デバイスまたはイベントを選択してください。",

		deviceDetailsTitle:  "デバイス",
		deviceMonitoring:    "監視",
		deviceName:          "名前",
		devicePowerState:    "電源状態",
		deviceManufacturer:  "製造元",
		deviceLocation:      "場所",
		deviceLastSeen:      "最終確認",
		eventDetailsTitle:   "イベント",
		eventMark:           "重要表示",
		eventTime:           "時刻",
		eventType:           "種別",
		eventConfidence:     "信頼度",
		eventMessage:        "メッセージ",
		rawETWProperties:    "Raw ETWプロパティ",
		markSuspend:         "! Suspend",
		markResume:          "Resume",
		markError:           "エラー",
		markPnPArrival:      "PnP +",
		markPnPRemoval:      "PnP -",
		exportDialogTitle:   "表示ログをJSONLで出力",
		exportFailedTitle:   "出力失敗",
		exportCompleteTitle: "出力完了",
		trayShow:            "表示",
		trayOpenLogs:        "ログフォルダ",
		trayExit:            "終了",
		trayTooltip:         "USB Suspend Watch - 監視中",
		notifyResumeTitle:   "USB Resume",
		notifySuspendTitle:  "USB Suspend疑い",
		unknownUSBDevice:    "USBデバイス",
		monitorOn:           "ON",
		monitorOff:          "OFF",

		statusText: map[string]string{
			statusSimpleMonitorRunning: "簡易監視中",
			statusETWDisabled:          "ETWヘルパーはリリースUIで無効",
			statusETWAlreadyRequested:  "ETWヘルパーは要求済み",
			statusETWWaiting:           "ETWヘルパーイベント待機中",
			statusETWReceived:          "ETWヘルパーイベント受信",
		},
	}
}

func englishStrings() uiStrings {
	return uiStrings{
		refreshButton:       "Refresh",
		startETWButton:      "Start ETW experimental",
		stopETWButton:       "Stop ETW",
		openLogsButton:      "Open logs",
		exportVisibleButton: "Export visible",
		languageLabel:       "Language",
		languageOptions:     []string{"Japanese", "English"},

		monitoringStatusTitle: "Monitoring status",
		monitoringPrefix:      "Monitoring",
		privilegePrefix:       "Privilege",
		privilegeChecking:     "checking",
		standardUser:          "standard user",
		administrator:         "administrator",
		preciseETWRequested:   "precise ETW requested",
		summaryFormat:         "USB: %d | Monitored: %d | Low power: %d | Suspected: %d | Resume: %d | Visible: %d",
		logPrefix:             "Log",

		connectedDevicesTitle: "Connected USB devices",
		timelineTitle:         "Suspend / Resume timeline",
		typeLabel:             "Type",
		confidenceLabel:       "Confidence",
		searchCue:             "Search device, VID/PID, Instance ID, message",
		typeOptions:           []string{"All", "Suspected suspend", "Resume", "PnP", "Error"},
		confidenceOptions:     []string{"All", "High+Medium", "High only"},
		deviceColumnTitles:    []string{"Monitor / Name", "VID/PID", "Power", "Enumerator", "Location", "Last seen"},
		eventColumnTitles:     []string{"Mark", "Time", "Event", "Confidence", "Source", "Device", "Message"},
		emptyDetails:          "Select a device or event to inspect details.",

		deviceDetailsTitle:  "Device",
		deviceMonitoring:    "Monitoring",
		deviceName:          "Name",
		devicePowerState:    "Power state",
		deviceManufacturer:  "Manufacturer",
		deviceLocation:      "Location",
		deviceLastSeen:      "Last seen",
		eventDetailsTitle:   "Event",
		eventMark:           "Mark",
		eventTime:           "Time",
		eventType:           "Type",
		eventConfidence:     "Confidence",
		eventMessage:        "Message",
		rawETWProperties:    "Raw ETW properties",
		markSuspend:         "! Suspend",
		markResume:          "Resume",
		markError:           "Error",
		markPnPArrival:      "PnP +",
		markPnPRemoval:      "PnP -",
		exportDialogTitle:   "Export visible JSONL log",
		exportFailedTitle:   "Export failed",
		exportCompleteTitle: "Export complete",
		trayShow:            "Show",
		trayOpenLogs:        "Open logs",
		trayExit:            "Exit",
		trayTooltip:         "USB Suspend Watch - monitoring",
		notifyResumeTitle:   "USB Resume",
		notifySuspendTitle:  "USB Suspend suspected",
		unknownUSBDevice:    "USB device",
		monitorOn:           "On",
		monitorOff:          "Off",

		statusText: map[string]string{
			statusSimpleMonitorRunning: "simple monitor running",
			statusETWDisabled:          "ETW helper disabled in release UI",
			statusETWAlreadyRequested:  "ETW helper is already requested",
			statusETWWaiting:           "waiting for ETW helper events",
			statusETWReceived:          "ETW helper events received",
		},
	}
}

func localizeStatus(text string, language displayLanguage) string {
	strings := stringsFor(language)
	if localized, ok := strings.statusText[text]; ok {
		return localized
	}
	return text
}
