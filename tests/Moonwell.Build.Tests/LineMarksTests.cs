namespace Moonwell.Build.Tests
{
    public sealed class LineMarksTests : IDisposable
    {
        private static readonly string[] Debug = { "DEBUG" };

        private readonly TestFolders folders;

        public LineMarksTests(ITestOutputHelper output)
        {
            folders = new TestFolders(output);
        }

        [Fact]
        public void marks_each_statement()
        {
            var source = Lines(
                "class A",
                "{",
                "    void F()",
                "    {",
                "        G();",
                "        H();",
                "    }",
                "}");

            var marked = LineMarks.Mark(source, 1, Debug);

            Assert.Contains("        /*[[--@mw 1:5]]*/ G();", marked);
            Assert.Contains("        /*[[--@mw 1:6]]*/ H();", marked);
            Assert.Equal(source.Split('\n').Length, marked.Split('\n').Length);
        }

        [Fact]
        public void marks_body_end()
        {
            var source = Lines(
                "class A",
                "{",
                "    void F()",
                "    {",
                "        G();",
                "    }",
                "}");

            var marked = LineMarks.Mark(source, 2, Debug);

            Assert.Contains("    /*[[--@mw -]]*/ }", marked);
        }

        [Fact]
        public void lambda_end_marks_enclosing()
        {
            var source = Lines(
                "using System;",
                "class A",
                "{",
                "    void F()",
                "    {",
                "        G();",
                "        Action act = () =>",
                "        {",
                "            H();",
                "        };",
                "    }",
                "}");

            var marked = LineMarks.Mark(source, 1, Debug);

            Assert.Contains("            /*[[--@mw 1:9]]*/ H();", marked);
            Assert.Contains("        /*[[--@mw 1:7]]*/ };", marked);
            Assert.Contains("    /*[[--@mw -]]*/ }", marked);
        }

        [Fact]
        public void two_statements_one_line()
        {
            var source = Lines(
                "class A",
                "{",
                "    void F()",
                "    {",
                "        G(); H();",
                "    }",
                "}");

            var marked = LineMarks.Mark(source, 1, Debug);

            Assert.Contains("        /*[[--@mw 1:5]]*/ G(); /*[[--@mw 1:5]]*/ H();", marked);
        }

        [Fact]
        public void switch_section()
        {
            var source = Lines(
                "class A",
                "{",
                "    void F(int x)",
                "    {",
                "        switch (x)",
                "        {",
                "            case 1:",
                "                G();",
                "                break;",
                "        }",
                "    }",
                "}");

            var marked = LineMarks.Mark(source, 1, Debug);

            Assert.Contains("/*[[--@mw 1:8]]*/ G();", marked);
            Assert.Contains("/*[[--@mw 1:9]]*/ break;", marked);
        }

        [Fact]
        public void inactive_branch()
        {
            var source = Lines(
                "class A",
                "{",
                "    void F()",
                "    {",
                "#if DEBUG",
                "        G();",
                "#else",
                "        H();",
                "#endif",
                "    }",
                "}");

            var development = LineMarks.Mark(source, 1, Debug);
            var release = LineMarks.Mark(source, 1, Array.Empty<string>());

            Assert.Contains("/*[[--@mw 1:6]]*/ G();", development);
            Assert.DoesNotContain("/*[[--@mw 1:8]]*/", development);
            Assert.Contains("/*[[--@mw 1:8]]*/ H();", release);
            Assert.DoesNotContain("/*[[--@mw 1:6]]*/", release);
        }

        [Fact]
        public void stage_copies_and_marks()
        {
            var folder = folders.TemporaryCopyOfSample();
            var generated = Path.Combine(folder, "build", "generated");
            Directory.CreateDirectory(generated);
            File.WriteAllText(Path.Combine(generated, "MapMain.g.cs"), Lines("class M", "{", "    static void Main()", "    {", "        N();", "    }", "}"));
            var project = MapProject.Find(folder);

            var marked = MarkedProject.Stage(project, Debug);

            Assert.Equal(Path.Combine(folder, "build", "marked"), marked.Folder);
            Assert.Equal(new[] { "Hello.cs", "Map.cs", "Objects.cs" }, marked.Files);
            Assert.Equal(Path.Combine(marked.Folder, "Sample.csproj"), marked.ProjectFile);
            var csproj = File.ReadAllText(marked.ProjectFile);
            var moonwell = Path.GetFullPath(Path.Combine(folder, "..", "..", "..", "..", "src", "Moonwell", "Moonwell.csproj"));
            Assert.Contains($"Include=\"{moonwell}\"", csproj);
            Assert.Contains("/*[[--@mw 1:", File.ReadAllText(Path.Combine(marked.Folder, "Hello.cs")));
            Assert.DoesNotContain("@mw", File.ReadAllText(Path.Combine(marked.Folder, "build", "generated", "MapMain.g.cs")));
            Assert.Equal(
                "error at " + Path.Combine(folder, "Hello.cs"),
                marked.Unstage("error at " + Path.Combine(marked.Folder, "Hello.cs")));
            Assert.Equal(
                "(" + Path.Combine(folder, "Bad.cs") + "@7:42)",
                marked.Unstage("(" + marked.Folder.Replace('\\', '/') + "\\Bad.cs@7:42)"));
        }

        [Fact]
        public void stage_empties_the_marked_folder()
        {
            var folder = folders.TemporaryCopyOfSample();
            var stale = Path.Combine(folder, "build", "marked", "Stale.cs");
            Directory.CreateDirectory(Path.GetDirectoryName(stale)!);
            File.WriteAllText(stale, "class Stale { }");

            MarkedProject.Stage(MapProject.Find(folder), Debug);

            Assert.False(File.Exists(stale));
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        private static string Lines(params string[] lines)
        {
            return string.Join("\n", lines) + "\n";
        }
    }
}
