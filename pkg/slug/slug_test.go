package slug

import "testing"

func TestValidate(t *testing.T) {
	valid := []string{
		"abc",
		"my-slug",
		"my_slug",
		"a-long-slug-with-many-parts-123",
		"welcome-email",
		"a1b",
		"tenant-code-01",
	}

	for _, s := range valid {
		if err := Validate(s); err != nil {
			t.Errorf("Validate(%q) returned error: %v", s, err)
		}
	}
}

func TestValidate_TooShort(t *testing.T) {
	shorts := []string{"", "a", "ab"}
	for _, s := range shorts {
		if err := Validate(s); err == nil {
			t.Errorf("Validate(%q) should fail (too short)", s)
		}
	}
}

func TestValidate_TooLong(t *testing.T) {
	// 65 chars: starts with 'a', 63 x 'b', ends with 'c'
	buf := make([]byte, 65)
	buf[0] = 'a'
	for i := 1; i < 64; i++ {
		buf[i] = 'b'
	}
	buf[64] = 'c'
	s := string(buf)
	if err := Validate(s); err == nil {
		t.Errorf("Validate(%q) should fail (too long, len=%d)", s, len(s))
	}
}

func TestValidate_InvalidChars(t *testing.T) {
	invalid := []string{
		"My-Slug",
		"my slug",
		"my.slug",
		"my@slug",
		"my/slug",
	}
	for _, s := range invalid {
		if err := Validate(s); err == nil {
			t.Errorf("Validate(%q) should fail (invalid chars)", s)
		}
	}
}

func TestValidate_StartsWithDigit(t *testing.T) {
	if err := Validate("1abc"); err == nil {
		t.Error("Validate(\"1abc\") should fail (starts with digit)")
	}
}

func TestValidate_EndsWithHyphen(t *testing.T) {
	if err := Validate("abc-"); err == nil {
		t.Error("Validate(\"abc-\") should fail (ends with hyphen)")
	}
}

func TestValidate_EndsWithUnderscore(t *testing.T) {
	if err := Validate("abc_"); err == nil {
		t.Error("Validate(\"abc_\") should fail (ends with underscore)")
	}
}

func TestValidate_ReservedWords(t *testing.T) {
	reserved := []string{
		"system",
		"admin",
		"api",
		"internal",
		"global",
		"null",
		"undefined",
	}
	for _, s := range reserved {
		if err := Validate(s); err == nil {
			t.Errorf("Validate(%q) should fail (reserved word)", s)
		}
	}
}

func TestValidate_MaxLength(t *testing.T) {
	// Exactly 64 chars should be valid
	buf := make([]byte, 64)
	buf[0] = 'a'
	for i := 1; i < 63; i++ {
		buf[i] = 'b'
	}
	buf[63] = 'c'
	s := string(buf)
	if err := Validate(s); err != nil {
		t.Errorf("Validate(%q) should pass (exactly 64 chars, len=%d): %v", s, len(s), err)
	}
}
