def is_palindrome(s):
    cleaned = ''.join(s.lower().split())
    return cleaned == cleaned[::-1]
