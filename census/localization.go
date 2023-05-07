package census

type Locale string

var DefaultLocale Locale = "en"

type Localization map[Locale]string

// type Localization struct {
// 	En string `json:"en,omitempty"`
// 	De string `json:"de,omitempty"`
// 	Es string `json:"es,omitempty"`
// 	Fr string `json:"fr,omitempty"`
// 	It string `json:"it,omitempty"`
// 	Ko string `json:"ko,omitempty"`
// 	Pt string `json:"pt,omitempty"`
// 	Ru string `json:"ru,omitempty"`
// 	Tr string `json:"tr,omitempty"`
// 	Zh string `json:"zh,omitempty"`
// }

func (l Localization) String() string { return l[DefaultLocale] }
