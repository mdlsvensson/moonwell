using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class MapSettingsTests : IDisposable
    {
        private readonly List<string> temporaryFolders = new List<string>();

        [Fact]
        public void reads_the_sample()
        {
            var project = MapProject.Find(Fixture("Sample"));
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);

            var settings = MapSettingsReader.Read(assembly, project);

            Assert.Equal("Sample.SampleMap", settings.TypeName);
            Assert.Equal("Sample map", settings.Name);
            Assert.Equal("Moonwell", settings.Author);
            Assert.Equal("Built by the tests.", settings.Description);
            Assert.Equal("Any", settings.RecommendedPlayers);
        }

        [Fact]
        public void no_map_class_is_an_error()
        {
            var project = MapProject.Find(TemporaryCopyOfNoMap());
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);

            var error = Assert.Throws<BuildException>(() => MapSettingsReader.Read(assembly, project));

            Assert.Equal("no map class: declare one class that derives from Moonwell.Map", error.Message);
        }

        [Fact]
        public void two_map_classes_are_an_error()
        {
            AssertReadFails("TwoMaps", "more than one map class: TwoMaps.First, TwoMaps.Second");
        }

        [Fact]
        public void declared_main_is_an_error()
        {
            AssertReadFails("DeclaresMain", "DeclaresMain.Program.Main: a map declares no Main; the build writes it");
        }

        [Fact]
        public void game_call_names_line()
        {
            AssertReadFails(
                "CallsGame",
                "Map.cs:9: CallsGame.GameMap calls the game while it is declared; game work starts from a reaction or the game's start");
        }

        [Fact]
        public void throwing_constructor_names_line()
        {
            AssertReadFails("Throws", "Map.cs:9: Throws.GameMap's constructor threw: no name");
        }

        [Fact]
        public void failed_dotnet_build_is_an_error()
        {
            var folder = TemporaryCopyOfNoMap();
            File.WriteAllText(Path.Combine(folder, "Broken.cs"), "namespace NoMap { public sealed class Broken { int }");
            var project = MapProject.Find(folder);
            var log = new StringWriter();

            var error = Assert.Throws<BuildException>(() => DotnetBuild.Run(project, false, log));

            Assert.Equal("dotnet build failed", error.Message);
            Assert.Contains("error CS", log.ToString());
        }

        public void Dispose()
        {
            foreach (var folder in temporaryFolders)
            {
                if (Directory.Exists(folder))
                {
                    Directory.Delete(folder, true);
                }
            }
        }

        private static string Fixture(string name)
        {
            return Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", name));
        }

        private static void AssertReadFails(string fixture, string message)
        {
            var project = MapProject.Find(Fixture(fixture));
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);

            var error = Assert.Throws<BuildException>(() => MapSettingsReader.Read(assembly, project));

            Assert.Equal(message, error.Message);
        }

        // A copy of NoMap beside the other fixtures (so its relative reference to Moonwell holds), with the
        // World Editor map file the project needs to be found.
        private string TemporaryCopyOfNoMap()
        {
            var folder = Path.Combine(Fixture("."), "Temp-" + Guid.NewGuid().ToString("N"));
            temporaryFolders.Add(folder);
            Directory.CreateDirectory(Path.Combine(folder, "map"));
            foreach (var file in Directory.GetFiles(Fixture("NoMap"), "*", SearchOption.TopDirectoryOnly))
            {
                File.Copy(file, Path.Combine(folder, Path.GetFileName(file)));
            }

            File.Copy(Path.Combine(Fixture("Sample"), "map", "war3map.w3i"), Path.Combine(folder, "map", "war3map.w3i"));
            return folder;
        }
    }
}
