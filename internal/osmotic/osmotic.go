// Package osmotic 实现 van't Hoff 渗透压关系 π = i·C·R·T。
//
// 全服务压力单位统一为 bar，浓度单位 mol/L，温度单位 K。
// 渗透压与膜侧工作压差使用同一套量纲，二者之间不做任何隐式换算。
package osmotic

import (
	"errors"
	"math"
)

// RBarL 是气体常数 R，单位 L·bar/(mol·K)。
const RBarL = 0.08314

var (
	// ErrNegativeMolarity 摩尔浓度为负。
	ErrNegativeMolarity = errors.New("osmotic: 摩尔浓度不能为负")
	// ErrNonPositiveTemperature 温度不为正。
	ErrNonPositiveTemperature = errors.New("osmotic: 温度必须为正开尔文数值（T>0）")
	// ErrNonPositiveFactor 范特霍夫因子不为正。
	ErrNonPositiveFactor = errors.New("osmotic: 范特霍夫因子 i 必须为正")
	// ErrNonFinite 输入不是有限数值。
	ErrNonFinite = errors.New("osmotic: 输入必须为有限数值，不能是 NaN 或 Inf")
)

// Pressure 按 van't Hoff 关系计算渗透压。
//
//	pi = i * molarity * R * T
//
// molarity 单位 mol/L，vantHoff 为范特霍夫因子（NaCl 取 2），
// tempK 单位 K，返回值单位 bar。
func Pressure(molarity, vantHoff, tempK float64) (float64, error) {
	if !finite(molarity) || !finite(vantHoff) || !finite(tempK) {
		return 0, ErrNonFinite
	}
	if molarity < 0 {
		return 0, ErrNegativeMolarity
	}
	if tempK <= 0 {
		return 0, ErrNonPositiveTemperature
	}
	if vantHoff <= 0 {
		return 0, ErrNonPositiveFactor
	}
	return vantHoff * molarity * RBarL * tempK, nil
}

func finite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
