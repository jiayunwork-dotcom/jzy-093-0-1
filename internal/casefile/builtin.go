package casefile

// fp 把字面量转成指针，供构造内置工况档使用。
func fp(v float64) *float64 { return &v }

// BrackishDemo 是内置的小型苦咸水淡化工况档。
//
// 进料为约 5.84 g/L（0.10 mol/L）NaCl，25 ℃；
// 手算渗透压 π = 2 × 0.10 × 0.08314 × 298.15 ≈ 4.96 bar，
// 落在“几个巴”的苦咸水量级。
func BrackishDemo() CaseFile {
	return CaseFile{
		Name:        "brackish-demo",
		Description: "内置苦咸水档：约5.84g/L NaCl，25°C，低回收苦咸水反渗透",
		Feed: FeedSpec{
			Concentration: Concentration{
				Solute:    "NaCl",
				Molarity:  fp(0.10),  // mol/L ≈ 5.84 g/L
				MassConc:  fp(5.844), // g/L
				MolarMass: fp(58.44), // g/mol
				VantHoff:  fp(2.0),
			},
			TemperatureK: 298.15,
			FlowLpH:      1000.0,
		},
		Membrane: MembraneSpec{
			AreaM2:       2.5,
			Permeability: 12.0, // L/(h·m²·bar)
		},
		Operating: OperatingSpec{
			AppliedPressureBar: 12.0,
			PolarizationFactor: 1.10,
			SaltRejection:      0.99,
		},
	}
}

// DefaultRegistry 返回登记了内置苦咸水档的登记表。
func DefaultRegistry() *Registry {
	r := NewRegistry()
	_ = r.Register(BrackishDemo())
	return r
}
