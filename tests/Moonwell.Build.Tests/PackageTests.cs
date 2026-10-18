using System.IO.Compression;
using System.Text.RegularExpressions;
using System.Xml.Linq;
using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class PackageTests : IClassFixture<PackedMoonwell>, IDisposable
    {
        private readonly PackedMoonwell packed;
        private readonly TestFolders folders = new TestFolders();

        public PackageTests(PackedMoonwell packed)
        {
            this.packed = packed;
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void package_has_source()
        {
            using var archive = ZipFile.OpenRead(packed.MoonwellPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell/Core/Runtime.cs", names);
            Assert.Contains("buildTransitive/Moonwell.props", names);
            Assert.Contains("buildTransitive/Moonwell.targets", names);
            Assert.Contains("analyzers/dotnet/cs/Moonwell.Analyzers.dll", names);
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var files = nuspec.Descendants().Where(element => element.Name.LocalName == "files").ToList();
            var runtime = files.Single(file => file.Attribute("include")!.Value.Replace('\\', '/') == "cs/net10.0/Moonwell/Core/Runtime.cs");
            Assert.Equal("None", runtime.Attribute("buildAction")!.Value);
        }

        [Fact]
        public void natives_package_has_source()
        {
            var natives = Path.Combine(packed.Feed, "Moonwell.Natives." + packed.Version + ".nupkg");
            using var archive = ZipFile.OpenRead(natives);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains(names, name => name.StartsWith("contentFiles/cs/net10.0/Moonwell.Natives/Generated/", StringComparison.Ordinal) && name.EndsWith(".cs", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));
        }

        [Fact]
        public void objects_package_has_source_and_depends_on_moonwell()
        {
            using var archive = ZipFile.OpenRead(packed.ObjectsPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Objects/Core/Declared.cs", names);
            Assert.Contains(names, name => name.StartsWith("contentFiles/cs/net10.0/Moonwell.Objects/Generated/Units/", StringComparison.Ordinal) && name.EndsWith(".cs", StringComparison.Ordinal));
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Objects.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Objects.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var files = nuspec.Descendants().Where(element => element.Name.LocalName == "files").ToList();
            var declared = files.Single(file => file.Attribute("include")!.Value.Replace('\\', '/') == "cs/net10.0/Moonwell.Objects/Core/Declared.cs");
            Assert.Equal("None", declared.Attribute("buildAction")!.Value);
            var dependency = Assert.Single(nuspec.Descendants(), element => element.Name.LocalName == "dependency" && element.Attribute("id")!.Value == "Moonwell");
            Assert.Equal(packed.Version, dependency.Attribute("version")!.Value);
            Assert.DoesNotContain("Analyzers", dependency.Attribute("exclude")?.Value ?? string.Empty);
        }

        [Fact]
        public void packaged_sample_builds()
        {
            var copy = PackagedSample(false, false);

            var log = new StringWriter();
            var archive = MapBuild.Run(copy, new BuildOptions(), log);

            Assert.True(File.Exists(archive), log.ToString());
            var script = File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua"));
            Assert.Contains("Moonwell.Runtime", script);
            Assert.Contains("sample: started (development)", script);
        }

        [Fact]
        public void packaged_sample_runs_the_analyzer()
        {
            var copy = PackagedSample(false, false);
            File.WriteAllText(
                Path.Combine(copy, "Desync.cs"),
                "namespace Sample\n{\n    public static class Desync\n    {\n        public static object Local() => Moonwell.Natives.GetLocalPlayer();\n    }\n}\n");

            var log = new StringWriter();
            Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), log));

            Assert.Contains("Desync.cs(5,41): error MW0002", log.ToString());
        }

        [Fact]
        public void packaged_objects_build()
        {
            var copy = PackagedSample(false, true);

            var log = new StringWriter();
            var archive = MapBuild.Run(copy, new BuildOptions(), log);

            Assert.True(File.Exists(archive), log.ToString());
            var script = File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua"));
            Assert.Contains("namespace.class(\"Paladin\"", script);
            Assert.Contains("[\"Sample.Arthas\"]", script);
            Assert.DoesNotContain("Archmage", script);
            Assert.True(File.Exists(Path.Combine(copy, "ids.json")));
        }

        [Fact]
        public void second_build_without_own_remove()
        {
            var copy = PackagedSample(true, true);

            var first = new StringWriter();
            MapBuild.Run(copy, new BuildOptions(), first);
            Assert.True(File.Exists(Path.Combine(copy, "build", "generated", "MapMain.g.cs")), first.ToString());

            var second = new StringWriter();
            var archive = MapBuild.Run(copy, new BuildOptions(), second);

            Assert.True(File.Exists(archive), second.ToString());
        }

        [Fact]
        public void package_folder_elsewhere_is_reported()
        {
            var copy = folders.TemporaryCopyOfSample();
            WriteAssets(copy, "Moonwell/0.0.0-missing", "package");

            var error = Assert.Throws<BuildException>(() => PackageSources.Check(MapProject.Find(copy)));

            var packages = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".nuget", "packages");
            Assert.Equal($"Moonwell 0.0.0-missing has no source in {packages}; CSharp.lua reads packages only from there (is NUGET_PACKAGES set?)", error.Message);
        }

        [Fact]
        public void other_packages_and_project_references_pass()
        {
            var copy = folders.TemporaryCopyOfSample();
            WriteAssets(copy, "Moonwell/0.0.0-missing", "project");

            PackageSources.Check(MapProject.Find(copy));

            WriteAssets(copy, "War3Net.Build/0.0.0-missing", "package");

            PackageSources.Check(MapProject.Find(copy));
        }

        private string PackagedSample(bool withoutOwnRemove, bool withObjects)
        {
            var copy = folders.TemporaryCopyOfSampleOutsideRepository();
            var project = Path.Combine(copy, "Sample.csproj");
            var text = File.ReadAllText(project);
            var references = Regex.Replace(text, @"<ProjectReference [^>]*Moonwell\.Objects\.csproj"" />", withObjects ? $"<PackageReference Include=\"Moonwell.Objects\" Version=\"{packed.Version}\" />" : "<!-- no objects -->");
            var replaced = Regex.Replace(references, @"<ProjectReference [^>]*Moonwell\.csproj"" />", $"<PackageReference Include=\"Moonwell\" Version=\"{packed.Version}\" />");
            Assert.NotEqual(text, replaced);
            if (!withObjects)
            {
                File.Delete(Path.Combine(copy, "Objects.cs"));
                File.Delete(Path.Combine(copy, "ids.json"));
            }

            if (withoutOwnRemove)
            {
                var removed = Regex.Replace(replaced, @"[ \t]*<Compile Remove=""build/\*\*"" />\r?\n", string.Empty);
                Assert.NotEqual(replaced, removed);
                replaced = removed;
            }

            File.WriteAllText(project, replaced);
            File.WriteAllText(
                Path.Combine(copy, "nuget.config"),
                "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"
                + "<configuration>\n"
                + "  <packageSources>\n"
                + "    <clear />\n"
                + $"    <add key=\"moonwell-test\" value=\"{packed.Feed}\" />\n"
                + "  </packageSources>\n"
                + "</configuration>\n");
            return copy;
        }

        private static void WriteAssets(string folder, string library, string type)
        {
            var obj = Path.Combine(folder, "obj");
            Directory.CreateDirectory(obj);
            File.WriteAllText(
                Path.Combine(obj, "project.assets.json"),
                "{ \"version\": 3, \"libraries\": { \"" + library + "\": { \"type\": \"" + type + "\" } } }");
        }
    }
}
