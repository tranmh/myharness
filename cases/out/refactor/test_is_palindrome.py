import unittest


def is_palindrome(s):
    cleaned = ''.join(s.lower().split())
    return cleaned == cleaned[::-1]


class TestIsPalindrome(unittest.TestCase):

    def test_simple_palindrome(self):
        self.assertTrue(is_palindrome("racecar"))

    def test_simple_non_palindrome(self):
        self.assertFalse(is_palindrome("hello"))

    def test_mixed_case_palindrome(self):
        self.assertTrue(is_palindrome("RaceCar"))

    def test_phrase_with_spaces(self):
        self.assertTrue(is_palindrome("A man a plan a canal Panama"))

    def test_single_character(self):
        self.assertTrue(is_palindrome("a"))

    def test_empty_string(self):
        self.assertTrue(is_palindrome(""))

    def test_two_same_chars(self):
        self.assertTrue(is_palindrome("aa"))

    def test_two_different_chars(self):
        self.assertFalse(is_palindrome("ab"))

    def test_spaces_only(self):
        self.assertTrue(is_palindrome("   "))

    def test_case_insensitive_non_palindrome(self):
        self.assertFalse(is_palindrome("Hello World"))

    def test_palindrome_with_multiple_spaces(self):
        self.assertTrue(is_palindrome("never  odd  or  even"))

    def test_all_same_letters(self):
        self.assertTrue(is_palindrome("aaaa"))


if __name__ == "__main__":
    unittest.main()
