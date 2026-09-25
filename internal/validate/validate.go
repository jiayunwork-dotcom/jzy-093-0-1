// Package validate 集中处理输入合法性、浓度量纲自洽与盐物料守恒校验。
//
// 它不负责任何过程计算，只负责“拒绝非法输入”和“核对守恒”，
// 以便与渗透压、膜通量等纯计算模块彼此独立。
package validate

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"rocalc/internal/casefile"
)

// 自洽性与守恒容差。
const (
	MolarityRelTol = 1e-6 // 两种浓度表示换算的相对容差
	MolarityAbsTol = 1e-9 // 换算的绝对容差（mol/L）
	BalanceRelTol  = 1e-9 // 盐守恒相对容差
)

// InputError 描述一组合法性错误，HTTP 层映射为 422。
type InputError struct{ Reasons []string }

func (e *InputError) Error() string { return "输入不合法：" + strings.Join(e.Reasons, "; ") }

// AsInputError 判断错误链中是否含有 *InputError。
func AsInputError(err error) (*InputError, bool) {
	var ie *InputError
	return ie, errors.As(err, &ie)
}

// BalanceError 表示盐物料守恒核对失败。
type BalanceError struct {
	FeedSalt    float64 // 进料盐流量，mol/h
	ProductSalt float64 // 产水盐流量，mol/h
	BrineSalt   float64 // 浓水盐流量，mol/h
	Residual    float64 // 残差 feed-(product+brine)，mol/h
	RelativeGap float64 // 相对残差
}

func (e *BalanceError) Error() string {
	return fmt.Sprintf("盐物料守恒不满足：进料 %.6g ≠ 产水 %.6g + 浓水 %.6g，残差 %.6g（相对 %.2e）",
		e.FeedSalt, e.ProductSalt, e.BrineSalt, e.Residual, e.RelativeGap)
}

// ResolveMolarity 把浓度输入解析为唯一的摩尔浓度（mol/L）。
//
// 规则：
//   - 至少给出摩尔浓度或“质量浓度+摩尔质量”中的一条路径；
//   - 质量浓度必须与正的摩尔质量成对出现；
//   - 浓度不能为负，摩尔质量必须为正；
//   - 两条路径同时给出时，换算结果必须在容差内自洽。
func ResolveMolarity(c casefile.Concentration) (float64, error) {
	var reasons []string
	switch {
	case c.Molarity == nil:
		// 未给出摩尔浓度，交由下方的“至少一条路径”检查
	case !isFinite(*c.Molarity):
		reasons = append(reasons, "摩尔浓度必须为有限数值")
	case *c.Molarity < 0:
		reasons = append(reasons, "摩尔浓度不能为负")
	}
	switch {
	case c.MassConc == nil:
	case !isFinite(*c.MassConc):
		reasons = append(reasons, "质量浓度必须为有限数值")
	case *c.MassConc < 0:
		reasons = append(reasons, "质量浓度不能为负")
	}
	switch {
	case c.MolarMass == nil:
	case !isFinite(*c.MolarMass):
		reasons = append(reasons, "摩尔质量必须为有限数值")
	case *c.MolarMass <= 0:
		reasons = append(reasons, "摩尔质量必须为正（g/mol）")
	}
	if c.VantHoff != nil && (!isFinite(*c.VantHoff) || *c.VantHoff <= 0) {
		reasons = append(reasons, "范特霍夫因子必须为正")
	}
	if len(reasons) > 0 {
		return 0, &InputError{Reasons: reasons}
	}

	hasMolar := c.Molarity != nil
	hasMass := c.MassConc != nil
	if !hasMolar && !hasMass {
		return 0, &InputError{Reasons: []string{"必须给出摩尔浓度，或给出质量浓度并配摩尔质量"}}
	}
	if hasMass && c.MolarMass == nil {
		return 0, &InputError{Reasons: []string{"给出质量浓度时必须同时给出正的摩尔质量（g/mol）"}}
	}

	if !hasMolar {
		// 仅质量路径：C_mol = 质量浓度(g/L) / 摩尔质量(g/mol)
		return *c.MassConc / *c.MolarMass, nil
	}
	if !hasMass {
		return *c.Molarity, nil
	}

	// 两条路径都在：必须自洽
	derived := *c.MassConc / *c.MolarMass
	if !molarityConsistent(*c.Molarity, derived) {
		return 0, &InputError{Reasons: []string{fmt.Sprintf(
			"摩尔浓度 %.8g mol/L 与质量浓度换算值 %.8g mol/L（%g g/L ÷ %g g/mol）不自洽",
			*c.Molarity, derived, *c.MassConc, *c.MolarMass)}}
	}
	return *c.Molarity, nil
}

func molarityConsistent(given, derived float64) bool {
	return math.Abs(given-derived) <= MolarityRelTol*math.Max(math.Abs(given), math.Abs(derived))+MolarityAbsTol
}

// CaseFile 对整份工况档做合法性检查，返回聚合后的 *InputError。
func CaseFile(c casefile.CaseFile) error {
	var reasons []string
	add := func(ok bool, msg string) {
		if !ok {
			reasons = append(reasons, msg)
		}
	}

	add(isFinite(c.Feed.TemperatureK) && c.Feed.TemperatureK > 0, "进料温度必须为正开尔文数值（T>0）")
	add(isFinite(c.Feed.FlowLpH) && c.Feed.FlowLpH > 0, "进料流量必须为正（L/h）")
	add(isFinite(c.Membrane.AreaM2) && c.Membrane.AreaM2 > 0, "膜面积必须为正（m²）")
	add(isFinite(c.Membrane.Permeability) && c.Membrane.Permeability > 0, "水力渗透系数必须为正")
	add(isFinite(c.Operating.AppliedPressureBar) && c.Operating.AppliedPressureBar >= 0, "工作压差不能为负（bar）")
	add(isFinite(c.Operating.PolarizationFactor) && c.Operating.PolarizationFactor >= 1, "浓差极化因子必须不小于 1")
	add(isFinite(c.Operating.SaltRejection) && c.Operating.SaltRejection >= 0 && c.Operating.SaltRejection <= 1,
		"盐截留率必须落在 [0,1]")

	mol, err := ResolveMolarity(c.Feed.Concentration)
	if err != nil {
		if ie, ok := AsInputError(err); ok {
			reasons = append(reasons, ie.Reasons...)
		}
	} else {
		_ = mol
	}

	if len(reasons) > 0 {
		return &InputError{Reasons: reasons}
	}
	return nil
}

// SaltStreams 是三股物流的盐流量（mol/h）与水流量（L/h）。
type SaltStreams struct {
	FeedFlow    float64
	ProductFlow float64
	BrineFlow   float64
	FeedConc    float64 // 进料摩尔浓度
	ProductConc float64 // 产水摩尔浓度
	BrineConc   float64 // 浓水摩尔浓度
}

// CheckSaltBalance 核对进料盐量 == 产水盐量 + 浓水盐量。
// 返回的 BalanceError（如有）供调用方带原因上报。
func CheckSaltBalance(s SaltStreams) *BalanceError {
	feed := s.FeedFlow * s.FeedConc
	prod := s.ProductFlow * s.ProductConc
	brine := s.BrineFlow * s.BrineConc
	residual := feed - (prod + brine)
	scale := math.Max(math.Abs(feed), 1.0)
	rel := math.Abs(residual) / scale
	if rel > BalanceRelTol {
		return &BalanceError{
			FeedSalt: feed, ProductSalt: prod, BrineSalt: brine,
			Residual: residual, RelativeGap: rel,
		}
	}
	return nil
}

func isFinite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
