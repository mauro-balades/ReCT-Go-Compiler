package binder

import (
	"ReCT-Go-Compiler/builtins"
	"ReCT-Go-Compiler/symbols"
)

type Conversion struct {
	Exists     bool
	IsIdentity bool
	IsImplicit bool
	IsExplicit bool
}

// constructor
func CreateConvertion(exists bool, isIdentity bool, isImplicit bool) Conversion {
	return Conversion{
		Exists:     exists,
		IsIdentity: isIdentity,
		IsImplicit: isImplicit,
		IsExplicit: exists && !isImplicit,
	}
}

// conversion types
var (
	NoConversion       = CreateConvertion(false, false, false) // conversion impossible / not allowed
	IdentityConversion = CreateConvertion(true, true, true)    // no conversion needed
	ImplicitConversion = CreateConvertion(true, false, true)   // automatic conversion
	ExplicitConversion = CreateConvertion(true, false, false)  // conversion will need to be explicitly specified by the user
)

func ClassifyConversion(from symbols.TypeSymbol, to symbols.TypeSymbol) Conversion {

	// if they are equal -> no conversion needed
	if from.Fingerprint() == to.Fingerprint() {
		return IdentityConversion
	}

	// converting to "any" is always allowed, even without a cast
	if from.Fingerprint() != builtins.Void.Fingerprint() &&
		to.Fingerprint() == builtins.Any.Fingerprint() {
		return ImplicitConversion
	}

	// converting from "any" is always allowed with a cast
	if from.Fingerprint() == builtins.Any.Fingerprint() &&
		to.Fingerprint() != builtins.Void.Fingerprint() {
		return ExplicitConversion
	}

	// converting from bool, int, or float to string
	if (from.Fingerprint() == builtins.Bool.Fingerprint() ||
		from.Fingerprint() == builtins.Int.Fingerprint() ||
		from.Fingerprint() == builtins.Float.Fingerprint()) &&
		to.Fingerprint() == builtins.String.Fingerprint() {
		return ExplicitConversion
	}

	// converting from a string to a bool, int, or float
	if from.Fingerprint() == builtins.String.Fingerprint() &&
		(to.Fingerprint() == builtins.Bool.Fingerprint() ||
			to.Fingerprint() == builtins.Int.Fingerprint() ||
			to.Fingerprint() == builtins.Float.Fingerprint()) {
		return ExplicitConversion
	}

	return NoConversion
}