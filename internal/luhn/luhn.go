package luhn

// LuhnValidator проверяет номера заказов по алгоритму Луна.
type LuhnValidator struct{}

// NewLuhnValidator создает новый валидатор номеров заказов.
func NewLuhnValidator() *LuhnValidator {
	return &LuhnValidator{}
}

// Valid возвращает true, если номер проходит проверку Луна.
func (v *LuhnValidator) Valid(number string) bool {
	sum := 0
	double := false

	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		if digit < 0 || digit > 9 {
			return false
		}

		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		double = !double
	}

	return sum > 0 && sum%10 == 0
}
