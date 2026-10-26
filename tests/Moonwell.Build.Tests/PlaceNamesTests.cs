namespace Moonwell.Build.Tests
{
    public sealed class PlaceNamesTests : IDisposable
    {
        private static readonly string[] Names = { "Bank", "Shop \"A\" \\ B" };

        private readonly TestFolders folders = new TestFolders();

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void known_name_passes()
        {
            var project = Project(("Map.cs", "class A { void F() { var a = AreaShape.Region(\"Bank\"); var b = Moonwell.AreaShape.Region(\"Bank\"); } }"));

            PlaceNames.Check(project, "AreaShape.Region", Names, "region");
        }

        [Fact]
        public void unknown_name_fails_with_line()
        {
            var source = string.Join("\n", "using Moonwell;", string.Empty, "class A", "{", "    void F()", "    {", "        var a = AreaShape.Region(\"Bnak\");", "    }", "}");
            var project = Project(("Map.cs", source));

            var error = Assert.Throws<BuildException>(() => PlaceNames.Check(project, "AreaShape.Region", Names, "region"));

            Assert.Equal("Map.cs:7: the map has no World Editor region named 'Bnak'. Its regions: Bank, Shop \"A\" \\ B.", error.Message);
        }

        [Fact]
        public void file_in_a_folder_is_named_with_slashes()
        {
            var project = Project(("Maps/Town.cs", "class A { void F() { AreaShape.Region(\"Gate\"); } }"));

            var error = Assert.Throws<BuildException>(() => PlaceNames.Check(project, "AreaShape.Region", Names, "region"));

            Assert.StartsWith("Maps/Town.cs:1: ", error.Message);
        }

        [Fact]
        public void escaped_name_matches()
        {
            var project = Project(("Map.cs", "class A { void F() { AreaShape.Region(\"Shop \\\"A\\\" \\\\ B\"); } }"));

            PlaceNames.Check(project, "AreaShape.Region", Names, "region");
        }

        [Fact]
        public void non_literal_is_skipped()
        {
            var project = Project(("Map.cs", "class A { void F(string name) { AreaShape.Region(name); AreaShape.Region(\"Gate\" + name); Other.Region(\"Gate\"); AreaShape.Circle(\"Gate\"); } }"));

            PlaceNames.Check(project, "AreaShape.Region", Names, "region");
        }

        [Fact]
        public void the_build_folder_is_skipped()
        {
            var project = Project(("build/generated/Other.cs", "class A { void F() { AreaShape.Region(\"Gate\"); } }"));

            PlaceNames.Check(project, "AreaShape.Region", Names, "region");
        }

        [Fact]
        public void a_map_with_no_regions_says_so()
        {
            var project = Project(("Map.cs", "class A { void F() { AreaShape.Region(\"Gate\"); } }"));

            var error = Assert.Throws<BuildException>(() => PlaceNames.Check(project, "AreaShape.Region", new string[0], "region"));

            Assert.EndsWith("Its regions: none.", error.Message);
        }

        [Fact]
        public void unknown_camera_fails_with_line()
        {
            var source = string.Join("\n", "using Moonwell;", string.Empty, "class A", "{", "    void F()", "    {", "        var a = CameraSetup.Named(\"Intro\");", "        var b = CameraSetup.Named(\"Entro\");", "    }", "}");
            var project = Project(("Map.cs", source));

            var error = Assert.Throws<BuildException>(() => PlaceNames.Check(project, "CameraSetup.Named", new[] { "Intro", "Boss" }, "camera"));

            Assert.Equal("Map.cs:8: the map has no World Editor camera named 'Entro'. Its cameras: Intro, Boss.", error.Message);
        }

        private MapProject Project(params (string Path, string Text)[] files)
        {
            var folder = folders.TemporaryFolderOutsideRepository();
            Directory.CreateDirectory(Path.Combine(folder, "map"));
            File.WriteAllText(Path.Combine(folder, "map", "war3map.w3i"), string.Empty);
            File.WriteAllText(Path.Combine(folder, "Test.csproj"), "<Project />");
            foreach (var (path, text) in files)
            {
                var full = Path.Combine(folder, path);
                Directory.CreateDirectory(Path.GetDirectoryName(full)!);
                File.WriteAllText(full, text);
            }

            return MapProject.Find(folder);
        }
    }
}
