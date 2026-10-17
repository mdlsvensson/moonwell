using Xunit;

namespace Moonwell.Build.Tests
{
    public sealed class MapProjectTests : IDisposable
    {
        private readonly List<string> temporaryFolders = new List<string>();

        [Fact]
        public void finds_the_sample()
        {
            var sample = Fixture("Sample");

            var project = MapProject.Find(sample);

            Assert.Equal("Sample", project.Name);
            Assert.Equal(sample, project.Folder);
            Assert.Equal(Path.Combine(sample, "Sample.csproj"), project.ProjectFile);
            Assert.Equal(Path.Combine(sample, "map"), project.MapFolder);
            Assert.Equal(Path.Combine(sample, "assets"), project.AssetsFolder);
            Assert.Equal(Path.Combine(sample, "build"), project.BuildFolder);
            Assert.Equal(Path.Combine(sample, "build", "Sample"), project.StagedFolder);
            Assert.Equal(Path.Combine(sample, "build", "Sample.w3x"), project.ArchivePath);
            Assert.Equal(Path.Combine(sample, "build", "generated"), project.GeneratedFolder);
        }

        [Fact]
        public void folder_forms_find_the_same_project()
        {
            var sample = Fixture("Sample");
            var expected = MapProject.Find(sample);
            var forms = new[]
            {
                sample + Path.DirectorySeparatorChar,
                Path.GetRelativePath(Directory.GetCurrentDirectory(), sample),
                Path.Combine(sample, "Sample.csproj"),
            };

            foreach (var form in forms)
            {
                var project = MapProject.Find(form);

                Assert.Equal(expected.Folder, project.Folder);
                Assert.Equal(expected.ProjectFile, project.ProjectFile);
            }
        }

        [Fact]
        public void no_project_is_an_error()
        {
            var folder = NewTemporaryFolder();

            var error = Assert.Throws<BuildException>(() => MapProject.Find(folder));

            Assert.Equal($"no C# project in {folder}", error.Message);
        }

        [Fact]
        public void two_projects_are_an_error()
        {
            var folder = NewTemporaryFolder();
            File.WriteAllText(Path.Combine(folder, "A.csproj"), string.Empty);
            File.WriteAllText(Path.Combine(folder, "B.csproj"), string.Empty);

            var error = Assert.Throws<BuildException>(() => MapProject.Find(folder));

            Assert.Equal($"more than one C# project in {folder}: A.csproj, B.csproj", error.Message);
        }

        [Fact]
        public void no_map_is_an_error()
        {
            var error = Assert.Throws<BuildException>(() => MapProject.Find(Fixture("NoMap")));

            Assert.Equal(@"no World Editor map: map\war3map.w3i is missing", error.Message);
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

        private string NewTemporaryFolder()
        {
            var folder = Path.Combine(Path.GetTempPath(), "moonwell-build-tests", Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(folder);
            temporaryFolders.Add(folder);
            return folder;
        }
    }
}
