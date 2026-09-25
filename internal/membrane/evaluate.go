package membrane

import (
	"errors"
	"fmt"

	"rocalc/internal/casefile"
	"rocalc/internal/osmotic"
	"rocalc/internal/validate"
)

// ResultUnits 声明结果中各物理量使用的统一单位。
type ResultUnits struct {
	Pressure        string  `json:"pressure"`
	Molarity        string  `json:"molality"`
	Temperature     string  `json:"temperature"`
	Flow            string  `json:"flow"`
	Area            string  `json:"area"`
	Permeability    string  `json:"permeability"`
	GasConstantBarL float64 `json:"gas_constant_L_bar_per_mol_K"`
}

// FeedResult 进料侧解析结果。
type FeedResult struct {
	Molarity     float64 `json:"molarity_mol_L"`
	TemperatureK float64 `json:"temperature_K"`
	FlowLpH      float64 `json:"feed_flow_L_p_h"`
	VantHoff     float64 `json:"van_t_hoff_factor"`
	PiBar        float64 `json:"feed_osmotic_pressure_bar"`
}

// OperatingResult 推动力与操作量。
type OperatingResult struct {
	AppliedPressureBar    float64 `json:"applied_pressure_bar"`
	PolarizationFactor    float64 `json:"polarization_factor"`
	SaltRejection         float64 `json:"salt_rejection"`
	OsmoticDeltaBar       float64 `json:"osmotic_pressure_difference_bar"`
	NetDrivingPressureBar float64 `json:"net_driving_pressure_bar"`
}

// MembraneResult 膜通量结果。
type MembraneResult struct {
	AreaM2         float64 `json:"area_m2"`
	Permeability   float64 `json:"hydraulic_permeability_L_p_h_p_m2_p_bar"`
	ProductFlowLpH float64 `json:"product_flow_L_p_h"`
	Recovery       float64 `json:"recovery"`
}

// SaltBalanceResult 盐衡算结果（均以摩尔浓度/摩尔流量计）。
type SaltBalanceResult struct {
	FeedMolarity     float64 `json:"feed_molarity_mol_L"`
	ProductMolarity  float64 `json:"product_molarity_mol_L"`
	BrineMolarity    float64 `json:"brine_molarity_mol_L"`
	BrineFlowLpH     float64 `json:"brine_flow_L_p_h"`
	FeedSaltMolpH    float64 `json:"feed_salt_mol_p_h"`
	ProductSaltMolpH float64 `json:"product_salt_mol_p_h"`
	BrineSaltMolpH   float64 `json:"brine_salt_mol_p_h"`
	ResidualMolpH    float64 `json:"residual_mol_p_h"`
}

// Result 一次完整工况核算的结果。计算过程是纯函数：同一份工况档
// 在任何地方计算都得到同一份结果，不同工况档互不影响。
type Result struct {
	CaseName  string            `json:"case_name"`
	Feed      FeedResult        `json:"feed"`
	Operating OperatingResult   `json:"operating"`
	Membrane  MembraneResult    `json:"membrane"`
	Salt      SaltBalanceResult `json:"salt_balance"`
	Units     ResultUnits       `json:"units"`
}

func defaultUnits() ResultUnits {
	return ResultUnits{
		Pressure:        "bar",
		Molarity:        "mol/L",
		Temperature:     "K",
		Flow:            "L/h",
		Area:            "m2",
		Permeability:    "L/(h*m2*bar)",
		GasConstantBarL: osmotic.RBarL,
	}
}

// Evaluate 对一份工况档执行完整核算。
//
// 顺序：合法性检查 → 浓度量纲解析 → 进料渗透压 → 跨膜渗透压差
// → NDP → 产水流量 → 回收率 → 三股浓度与流量 → 盐物料守恒核对。
// 任何一步非法都会带原因返回错误，绝不产出物理上不成立的结果。
func Evaluate(c casefile.CaseFile) (*Result, error) {
	if err := validate.CaseFile(c); err != nil {
		return nil, err
	}

	molarity, err := validate.ResolveMolarity(c.Feed.Concentration)
	if err != nil {
		return nil, err
	}
	i := 2.0
	if c.Feed.Concentration.VantHoff != nil {
		i = *c.Feed.Concentration.VantHoff
	}

	piF, err := osmotic.Pressure(molarity, i, c.Feed.TemperatureK)
	if err != nil {
		return nil, &DomainError{Err: err}
	}

	dPi := OsmoticDelta(piF, c.Operating.PolarizationFactor)
	ndp := NetDrivingPressure(c.Operating.AppliedPressureBar, dPi)

	switch {
	case ndp < 0:
		return nil, &DomainError{Err: fmt.Errorf("%w：Δp=%.6g bar，Δπ=%.6g bar，NDP=%.6g bar",
			ErrNegativeNDP, c.Operating.AppliedPressureBar, dPi, ndp)}
	case ndp == 0:
		return nil, &DomainError{Err: fmt.Errorf("%w：Δp=%.6g bar，Δπ=%.6g bar",
			ErrZeroNDP, c.Operating.AppliedPressureBar, dPi)}
	}

	qp, err := Flux(c.Membrane.AreaM2, c.Membrane.Permeability, ndp)
	if err != nil {
		return nil, err
	}
	y := Recovery(qp, c.Feed.FlowLpH)
	if y <= 0 || y >= 1 {
		return nil, &DomainError{Err: fmt.Errorf("%w：Y=%.8g（Qp=%.6g，Qf=%.6g）",
			ErrRecoveryOutOfRange, y, qp, c.Feed.FlowLpH)}
	}

	cb, err := BrineConcentration(molarity, y, c.Operating.SaltRejection)
	if err != nil {
		return nil, err
	}
	cp := 0.0
	if c.Operating.SaltRejection < 1 {
		cp = (1 - c.Operating.SaltRejection) * molarity
	}
	qb := c.Feed.FlowLpH - qp

	streams := validate.SaltStreams{
		FeedFlow: c.Feed.FlowLpH, ProductFlow: qp, BrineFlow: qb,
		FeedConc: molarity, ProductConc: cp, BrineConc: cb,
	}
	if berr := validate.CheckSaltBalance(streams); berr != nil {
		return nil, &DomainError{Err: errors.New(berr.Error())}
	}

	return &Result{
		CaseName: c.Name,
		Feed: FeedResult{
			Molarity:     molarity,
			TemperatureK: c.Feed.TemperatureK,
			FlowLpH:      c.Feed.FlowLpH,
			VantHoff:     i,
			PiBar:        piF,
		},
		Operating: OperatingResult{
			AppliedPressureBar:    c.Operating.AppliedPressureBar,
			PolarizationFactor:    c.Operating.PolarizationFactor,
			SaltRejection:         c.Operating.SaltRejection,
			OsmoticDeltaBar:       dPi,
			NetDrivingPressureBar: ndp,
		},
		Membrane: MembraneResult{
			AreaM2:         c.Membrane.AreaM2,
			Permeability:   c.Membrane.Permeability,
			ProductFlowLpH: qp,
			Recovery:       y,
		},
		Salt: SaltBalanceResult{
			FeedMolarity:     molarity,
			ProductMolarity:  cp,
			BrineMolarity:    cb,
			BrineFlowLpH:     qb,
			FeedSaltMolpH:    c.Feed.FlowLpH * molarity,
			ProductSaltMolpH: qp * cp,
			BrineSaltMolpH:   qb * cb,
			ResidualMolpH:    c.Feed.FlowLpH*molarity - (qp*cp + qb*cb),
		},
		Units: defaultUnits(),
	}, nil
}
