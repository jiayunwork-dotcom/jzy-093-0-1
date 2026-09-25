package validate

import (
	"errors"
	"math"
	"testing"

	"rocalc/internal/casefile"
)

func ptr(v float64) *float64 { return &v }

func TestResolveMolarityDirect(t *testing.T) {
	got, err := ResolveMolarity(casefile.Concentration{Molarity: ptr(0.1), VantHoff: ptr(2)})
	if err != nil || got != 0.1 {
		t.Fatalf("直接摩尔浓度路径 got=%v err=%v", got, err)
	}
}

func TestResolveMolarityFromMass(t *testing.T) {
	// 5.844 g/L ÷ 58.44 g/mol = 0.1000 mol/L
	got, err := ResolveMolarity(casefile.Concentration{
		MassConc: ptr(5.844), MolarMass: ptr(58.44), VantHoff: ptr(2),
	})
	if err != nil {
		t.Fatalf("质量路径意外错误：%v", err)
	}
	if math.Abs(got-0.1) > 1e-9 {
		t.Fatalf("质量换算摩尔浓度 = %.7f，期望 0.1", got)
	}
}

// TestResolveMolarityConsistency 两条浓度路径必须自洽，否则拒绝。
func TestResolveMolarityConsistency(t *testing.T) {
	ok := casefile.Concentration{
		Molarity: ptr(0.1000001), MassConc: ptr(5.844), MolarMass: ptr(58.44), VantHoff: ptr(2),
	}
	if _, err := ResolveMolarity(ok); err != nil {
		t.Fatalf("容差内应判定自洽：%v", err)
	}

	bad := casefile.Concentration{
		Molarity: ptr(0.5), MassConc: ptr(5.844), MolarMass: ptr(58.44), VantHoff: ptr(2),
	}
	if _, err := ResolveMolarity(bad); err == nil {
		t.Fatal("摩尔浓度 0.5 与质量换算 0.1 明显不自洽，必须拒绝")
	} else if ie, ok := AsInputError(err); !ok || len(ie.Reasons) == 0 {
		t.Fatalf("应返回带原因的 InputError，got %v", err)
	}
}

func TestResolveMolarityMissingAndNegative(t *testing.T) {
	if _, err := ResolveMolarity(casefile.Concentration{}); err == nil {
		t.Fatal("未给任何浓度必须拒绝")
	}
	if _, err := ResolveMolarity(casefile.Concentration{Molarity: ptr(-1)}); err == nil {
		t.Fatal("负摩尔浓度必须拒绝")
	}
	if _, err := ResolveMolarity(casefile.Concentration{MassConc: ptr(10)}); err == nil {
		t.Fatal("只给质量浓度不给摩尔质量必须拒绝")
	}
	if _, err := ResolveMolarity(casefile.Concentration{MassConc: ptr(10), MolarMass: ptr(0)}); err == nil {
		t.Fatal("摩尔质量非正必须拒绝")
	}
}

func TestCaseFileValidation(t *testing.T) {
	good := casefile.BrackishDemo()
	if err := CaseFile(good); err != nil {
		t.Fatalf("内置档应合法：%v", err)
	}

	bad := good
	bad.Feed.TemperatureK = -5
	bad.Operating.PolarizationFactor = 0.9
	bad.Operating.SaltRejection = 1.2
	err := CaseFile(bad)
	if err == nil {
		t.Fatal("非法工况档必须被拒绝")
	}
	ie, ok := AsInputError(err)
	if !ok || len(ie.Reasons) < 3 {
		t.Fatalf("应聚合全部非法原因（≥3 条），got %v", err)
	}
}

// TestSaltBalance 重点守恒：进料盐量必须等于产水盐量加浓水盐量。
func TestSaltBalance(t *testing.T) {
	// Cf=0.1，Y=0.2，r=0.99：Cp=0.001，Cb=0.1·(1-0.001·0.2)/0.8=0.124975
	s := SaltStreams{
		FeedFlow: 1000, ProductFlow: 200, BrineFlow: 800,
		FeedConc: 0.1, ProductConc: 0.001, BrineConc: 0.1 * (1 - 0.01*0.2) / 0.8,
	}
	if berr := CheckSaltBalance(s); berr != nil {
		t.Fatalf("守恒组合不应报错：%v", berr)
	}

	// 经典翻车写法 Cb = Cf·(1−Y) = 0.08：回收越高浓水越稀，守恒必崩。
	broken := s
	broken.BrineConc = 0.1 * (1 - 0.2)
	berr := CheckSaltBalance(broken)
	if berr == nil {
		t.Fatal("Cb=Cf·(1−Y) 的错误写法必须被守恒校验抓住")
	}
	var be *BalanceError
	if !errors.As(berr, &be) {
		t.Fatalf("应返回 *BalanceError，got %T", berr)
	}
}
