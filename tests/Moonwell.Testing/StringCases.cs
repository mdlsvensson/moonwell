namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for string literals. CSharp.lua copied a literal into the script as written, and Lua reads some escapes otherwise or
    /// refuses them (C4); an interpolated string's text and a % in one were written as they were.
    /// </summary>
    public static class StringCases
    {
        /// <summary>\e, \U, \x of one to four digits and \0 before a digit mean what C# says.</summary>
        public static void Escapes(FakeWorld world)
        {
            Expect.Equal(27, "a\eb"[1], "\\e is the escape character");
            Expect.Equal(3, "a\eb".Length, "\\e is one character");
            Expect.Equal("aAb", "a\U00000041b", "\\U with eight digits");
            Expect.Equal("aAg", "a\x41g", "\\x41 and a g that is no digit");
            Expect.Equal(27, "a\x1b"[1], "\\x1b");
            Expect.Equal(3, "a\x1bz".Length, "\\x1b then z");
            Expect.Equal(3, "a\01".Length, "\\0 then 1 is two characters");
            Expect.Equal(0, "a\01"[1], "\\0 is NUL");
            Expect.Equal(49, "a\01"[2], "the 1 after \\0 stays a 1");
            Expect.Equal(2, "a\x4".Length, "\\x4");
            Expect.Equal(4, "a\x4"[1], "\\x4 is the character 4");
        }

        /// <summary>A raw string is its text, and a verbatim one keeps its backslashes and quotes.</summary>
        public static void RawAndVerbatim(FakeWorld world)
        {
            Expect.Equal("a\\b", @"a\b", "a verbatim string keeps its backslash");
            Expect.Equal("a\"b", @"a""b", "a verbatim string's doubled quote");
            Expect.Equal("a\\b\"c", """a\b"c""", "a raw string");
            Expect.Equal("a]]b", @"a]]b", "a verbatim string with ]]");
            Expect.Equal("\r\n", "\r\n", "a carriage return and a line feed");
        }

        /// <summary>A UTF-8 literal is its bytes.</summary>
        public static void Utf8(FakeWorld world)
        {
            System.ReadOnlySpan<byte> bytes = "AB"u8;
            Expect.Equal(2, bytes.Length, "the length of \"AB\"u8");
            Expect.Equal(66, bytes[1], "its second byte");
        }

        /// <summary>The text of an interpolated string is escaped as a regular string's is.</summary>
        public static void Interpolated(FakeWorld world)
        {
            var n = Three();
            Expect.Equal("a\\3\"q", $"a\\{n}\"q", "a backslash and a quote in the text");
            Expect.Equal(5, $"a\\{n}\"q".Length, "their length");
            Expect.Equal(10, $"a\n{n}\t{n}\eb"[1], "a line feed in the text");
            Expect.Equal(27, $"a\n{n}\t{n}\eb"[5], "an escape in the text");
        }

        /// <summary>A % in a verbatim or raw interpolated string is a % (string.format needs it doubled).</summary>
        public static void Percent(FakeWorld world)
        {
            var n = Three();
            Expect.Equal("100% 3", $@"100% {n}", "a verbatim interpolated string");
            Expect.Equal("100% 3 \\x", $"""100% {n} \x""", "a raw interpolated string");
            Expect.Equal("50%%", @"50%%", "a verbatim string");
        }

        private const string Reset = "\x1b[0m";

        /// <summary>A const string and literals joined with +, which CSharp.lua writes by their value, mean what C# says.</summary>
        public static void Folded(FakeWorld world)
        {
            Expect.Equal(27, Reset[0], "a const string's \\x1b");
            Expect.Equal(4, Reset.Length, "a const string's length");
            Expect.Equal(2, ("\e" + "y").Length, "\\e joined to a literal");
            Expect.Equal(27, ("\e" + "y")[0], "\\e joined to a literal is the escape character");
            Expect.Equal(3, ("b\0" + "2").Length, "\\0 joined to a digit is two characters");
            Expect.Equal(50, ("b\0" + "2")[2], "the 2 after a joined \\0 stays a 2");
        }

        private static int Three()
        {
            return 3;
        }
    }
}
