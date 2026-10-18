namespace Moonwell.Build.Tests
{
    public sealed class LineTableTests
    {
        [Fact]
        public void drops_markers_and_maps_lines()
        {
            var script = "a\n--@mw 1:5\nb\nc\n  --@mw -\nd\n";

            var applied = LineTable.Apply(script, new[] { "X.cs" });

            Assert.Equal("a\nb\nc\nd\n\nMoonwellLines = { files = { \"X.cs\" }, map = \"2 1 5 2 0 0\" }\n", applied);
        }

        [Fact]
        public void keeps_crlf_line_ends()
        {
            var script = "a\r\n    --@mw 2:7\r\nb\r\n";

            var applied = LineTable.Apply(script, new[] { "X.cs", "Y.cs" });

            Assert.Equal("a\r\nb\r\n\r\nMoonwellLines = { files = { \"X.cs\", \"Y.cs\" }, map = \"2 2 7\" }\r\n", applied);
        }

        [Fact]
        public void later_marker_on_the_same_line_wins()
        {
            var script = "--@mw -\n--@mw 1:3\nb\n";

            var applied = LineTable.Apply(script, new[] { "X.cs" });

            Assert.EndsWith("map = \"1 1 3\" }\n", applied);
        }

        [Fact]
        public void marker_text_inside_a_line_is_kept()
        {
            var script = "x = \"--@mw 1:5\"\n-- --@mw 1:5\n";

            var applied = LineTable.Apply(script, new[] { "X.cs" });

            Assert.StartsWith("x = \"--@mw 1:5\"\n-- --@mw 1:5\n", applied);
            Assert.EndsWith("map = \"\" }\n", applied);
        }

        [Fact]
        public void paths_are_lua_strings()
        {
            var applied = LineTable.Apply("a\n", new[] { "we\"ird\\name.cs" });

            Assert.Contains("files = { \"we\\\"ird\\\\name.cs\" }", applied);
        }
    }
}
