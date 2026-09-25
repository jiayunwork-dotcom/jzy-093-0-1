// Package casefile 定义反渗透工况档的数据模型、具名登记表以及内置工况档。
//
// 工况档是纯数据：登记、取用与计算过程互不依赖共享状态，
// 同一进程内可以并存任意多份互不影响的工况档。
package casefile

// Concentration 描述进料溶质浓度。
//
// 摩尔浓度可直接给 Molarity；也可给质量浓度 MassConc 配摩尔质量 MolarMass，
// 两条路径同时给出时换算结果必须自洽（自洽性在 validate 包中检查）。
// 指针字段用于区分“未给出”与“给出零值”。
type Concentration struct {
	// Molarity 摩尔浓度，mol/L。
	Molarity *float64 `json:"molarity_mol_L,omitempty"`
	// MassConc 质量浓度，g/L。
	MassConc *float64 `json:"mass_concentration_g_L,omitempty"`
	// MolarMass 溶质摩尔质量，g/mol。
	MolarMass *float64 `json:"molar_mass_g_mol,omitempty"`
	// Solute 溶质名称，仅作标注，不参与计算（如 NaCl）。
	Solute string `json:"solute,omitempty"`
	// VantHoff 范特霍夫因子 i（NaCl 取 2）。
	VantHoff *float64 `json:"van_t_hoff_factor,omitempty"`
}

// FeedSpec 进料侧工况。
type FeedSpec struct {
	Concentration Concentration `json:"concentration"`
	// TemperatureK 进料温度，K，必须为正。
	TemperatureK float64 `json:"temperature_K"`
	// FlowLpH 进料流量 Qf，L/h。
	FlowLpH float64 `json:"feed_flow_L_p_h"`
}

// MembraneSpec 膜元件参数。
type MembraneSpec struct {
	// AreaM2 有效膜面积 A_mem，m²。
	AreaM2 float64 `json:"area_m2"`
	// Permeability 水力渗透系数 Lp，L/(h·m²·bar)。
	Permeability float64 `json:"hydraulic_permeability_L_p_h_p_m2_p_bar"`
}

// OperatingSpec 操作工况。
type OperatingSpec struct {
	// AppliedPressureBar 工作（跨膜液压）压差 Δp，bar。
	AppliedPressureBar float64 `json:"applied_pressure_bar"`
	// PolarizationFactor 浓差极化因子 β，>=1；β=1 表示无极化。
	PolarizationFactor float64 `json:"polarization_factor"`
	// SaltRejection 表观盐截留率 r，取值 [0,1]；r=1 表示产水不带盐。
	SaltRejection float64 `json:"salt_rejection"`
}

// CaseFile 一份具名反渗透工况档。
type CaseFile struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Feed        FeedSpec      `json:"feed"`
	Membrane    MembraneSpec  `json:"membrane"`
	Operating   OperatingSpec `json:"operating"`
}
