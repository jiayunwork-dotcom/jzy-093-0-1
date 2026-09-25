// Package membrane 实现反渗透膜侧的净推动力、产水通量、回收与盐衡算，
// 并把 osmotic / validate 两包编排成一次完整的工况核算。
//
// 全流程压力单位统一为 bar、流量 L/h、浓度 mol/L，
// 渗透压与工作压差之间不做任何隐式量纲换算。
package membrane

import (
	"errors"
	"math"
)

var (
	// ErrNegativeNDP 净推动力为负（工作压差已不足以克服渗透压差）。
	// 此时任何正产水通量都是非法结果。
	ErrNegativeNDP = errors.New("工作压差低于跨膜渗透压差，净推动力 NDP 为负，不能产生正产水通量")
	// ErrZeroNDP 净推动力恰好为零，产水流量为零，回收率落不在开区间 (0,1)。
	ErrZeroNDP = errors.New("工作压差恰好抵消跨膜渗透压差（无极化时 Δp=πf），产水流量为零")
	// ErrRecoveryOutOfRange 回收率不在开区间 (0,1)。
	ErrRecoveryOutOfRange = errors.New("回收率必须落在开区间 (0,1)")
)

// DomainError 携带可读原因的过程计算错误，HTTP 层映射为 422。
type DomainError struct{ Err error }

func (e *DomainError) Error() string { return e.Err.Error() }
func (e *DomainError) Unwrap() error { return e.Err }

// NetDrivingPressure 返回 NDP = Δp − Δπ。
func NetDrivingPressure(appliedPressureBar, osmoticDeltaBar float64) float64 {
	return appliedPressureBar - osmoticDeltaBar
}

// OsmoticDelta 返回膜面跨膜渗透压差 Δπ = β·πf − πp。
//
// 浓差极化用薄因子 β 表达：膜面进料侧浓度被放大为 β·Cf。
// 苦咸水淡化中 πp ≪ πf，忽略产水侧渗透压，即 πp=0。
func OsmoticDelta(feedPiBar, polarizationFactor float64) float64 {
	return polarizationFactor * feedPiBar
}

// Flux 按 Qp = A_mem·Lp·NDP 计算产水流量。
//
// area 单位 m²，permeability 单位 L/(h·m²·bar)，ndp 单位 bar，
// 返回 L/h。NDP 为负直接拒绝，绝不返回正通量。
func Flux(area, permeability, ndp float64) (float64, error) {
	if math.IsNaN(ndp) || math.IsInf(ndp, 0) {
		return 0, &DomainError{Err: errors.New("NDP 必须为有限数值")}
	}
	if ndp < 0 {
		return 0, &DomainError{Err: ErrNegativeNDP}
	}
	return area * permeability * ndp, nil
}

// Recovery 返回回收率 Y = Qp/Qf。合法性区间 (0,1) 由调用方用
// validate 包或 CheckRecovery 另行检查。
func Recovery(productFlow, feedFlow float64) float64 {
	return productFlow / feedFlow
}

// BrineConcentration 返回浓水摩尔浓度。
//
//   - r=1（完全截留）：产水不带盐，Cb = Cf/(1−Y)；
//   - r<1（表观截留）：产水带盐 Cp=(1−r)·Cf，
//     由物料守恒推出 Cb = Cf·(1−(1−r)·Y)/(1−Y)。
//
// r→1 时通式自然退化为极限式。绝不能写成 Cf·(1−Y)
// （那样回收越高浓水反而越稀，盐守恒必然不成立）。
func BrineConcentration(feedMolarity, recovery, rejection float64) (float64, error) {
	if recovery <= 0 || recovery >= 1 {
		return 0, &DomainError{Err: ErrRecoveryOutOfRange}
	}
	if rejection == 1 {
		return feedMolarity / (1 - recovery), nil
	}
	return feedMolarity * (1 - (1-rejection)*recovery) / (1 - recovery), nil
}
