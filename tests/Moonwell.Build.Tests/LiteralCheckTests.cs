namespace Moonwell.Build.Tests
{
    public sealed class LiteralCheckTests : IDisposable
    {
        private static readonly string[] Debug = { "DEBUG" };

        private readonly TestFolders folders = new TestFolders();

        [Theory]
        [InlineData(@"""a\eb""", @"X.cs:3: the escape \e does not compile to Lua; write \u001b")]
        [InlineData(@"""a\U0001F600b""", @"X.cs:3: the escape \U0001F600 does not compile to Lua; write the character itself")]
        [InlineData(@"""a\x4""", @"X.cs:3: the escape \x4 reads differently in Lua, which takes two hex digits; write \u0004")]
        [InlineData(@"""a\x41b""", @"X.cs:3: the escape \x41b reads differently in Lua, which takes two hex digits; write \u041B")]
        [InlineData(@"""a\x0041""", @"X.cs:3: the escape \x0041 reads differently in Lua, which takes two hex digits; write \u0041")]
        [InlineData(@"""a\012""", @"X.cs:3: the escape \0 before a digit reads as a decimal escape in Lua; write \u0000")]
        [InlineData(@"$""a\e{1}""", @"X.cs:3: the escape \e does not compile to Lua; write \u001b")]
        [InlineData("\"\"\"a\"\"\"", @"X.cs:3: a raw string literal does not compile to Lua; use a regular or verbatim string")]
        [InlineData(@"""a""u8", @"X.cs:3: a UTF-8 string literal (u8) does not compile to Lua")]
        public void reports_literals_lua_reads_otherwise(string literal, string message)
        {
            var source = Source(literal);

            Assert.Equal(message, string.Join(" | ", LiteralCheck.Find(source, "X.cs", Debug)));
        }

        [Theory]
        [InlineData(@"""a\x41\u00e9\0\t\\\""""")]
        [InlineData(@"""a\x41 b""")]
        [InlineData(@"@""a\eb""")]
        [InlineData(@"@""a]]b""")]
        [InlineData("$\"\"\"a{1}\"\"\"")]
        [InlineData(@"$@""a\e{1}""")]
        [InlineData(@"'\e'")]
        public void accepts_literals_lua_reads_the_same(string literal)
        {
            Assert.Empty(LiteralCheck.Find(Source(literal), "X.cs", Debug));
        }

        [Fact]
        public void verbatim_string_starting_with_a_line_end_is_reported()
        {
            var source = "class A\r\n{\r\n    object F() => @\"\r\nab\";\r\n}\r\n";

            Assert.Equal(
                new[] { @"X.cs:3: a verbatim string that starts with a line end loses it in Lua; write that line end as ""\r\n"" + ..." },
                LiteralCheck.Find(source, "X.cs", Debug));
        }

        [Fact]
        public void inactive_branch_is_not_checked()
        {
            var source = "class A\n{\n#if DEBUG\n    object F() => \"ok\";\n#else\n    object F() => \"a\\eb\";\n#endif\n}\n";

            Assert.Empty(LiteralCheck.Find(source, "X.cs", Debug));
            Assert.Single(LiteralCheck.Find(source, "X.cs", Array.Empty<string>()));
        }

        [Fact]
        public void build_reports_every_bad_literal()
        {
            var folder = folders.TemporaryCopyOfSample();
            File.WriteAllText(
                Path.Combine(folder, "Bad.cs"),
                "namespace Sample\n{\n    public static class Bad\n    {\n        public static string A = \"a\\eb\";\n        public static string B = \"a\\x41b\";\n    }\n}\n");

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(folder, new BuildOptions(), TextWriter.Null));

            Assert.Equal(
                @"Bad.cs:5: the escape \e does not compile to Lua; write \u001b" + Environment.NewLine
                + @"Bad.cs:6: the escape \x41b reads differently in Lua, which takes two hex digits; write \u041B",
                error.Message);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        private static string Source(string literal)
        {
            return "class A\n{\n    object F() => " + literal + ";\n}\n";
        }
    }
}
