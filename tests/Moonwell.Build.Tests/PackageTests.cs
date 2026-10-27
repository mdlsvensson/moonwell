using System.IO.Compression;
using System.Reflection.Metadata;
using System.Reflection.PortableExecutable;
using System.Text.RegularExpressions;
using System.Xml.Linq;
using War3Net.Build.Extensions;
using War3Net.Build.Info;
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
        public void package_source_has_its_own_version()
        {
            using var archive = ZipFile.OpenRead(packed.MoonwellPackage);
            var entry = archive.Entries.SingleOrDefault(candidate => candidate.FullName == "contentFiles/cs/net10.0/Moonwell/Core/Package.g.cs");
            Assert.NotNull(entry);
            using var reader = new StreamReader(entry!.Open());
            Assert.Contains("public const string Version = \"" + packed.Version + "\";", reader.ReadToEnd());
        }

        [Theory]
        [InlineData("Moonwell")]
        [InlineData("Moonwell.Natives")]
        [InlineData("Moonwell.Objects")]
        [InlineData("Moonwell.Damage")]
        [InlineData("Moonwell.Maths")]
        [InlineData("Moonwell.Interactions")]
        [InlineData("Moonwell.Terrain")]
        [InlineData("Moonwell.Physics")]
        [InlineData("Moonwell.Areas")]
        [InlineData("Moonwell.Input")]
        public void package_ships_xml_docs_beside_its_library(string id)
        {
            using var archive = ZipFile.OpenRead(Path.Combine(packed.Feed, id + "." + packed.Version + ".nupkg"));
            var docs = archive.Entries.SingleOrDefault(entry => entry.FullName == "lib/net10.0/" + id + ".xml");
            Assert.NotNull(docs);
            using var reader = new StreamReader(docs!.Open());
            var members = XDocument.Parse(reader.ReadToEnd()).Descendants("member").ToList();
            Assert.NotEmpty(members);
            Assert.DoesNotContain(members.Descendants("summary"), summary => summary.Value.Contains("@CSharpLua", StringComparison.Ordinal));
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
            Assert.Contains(names, name => name.StartsWith("contentFiles/cs/net10.0/Moonwell.Objects/Generated/Human/", StringComparison.Ordinal) && name.EndsWith(".cs", StringComparison.Ordinal));
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
        public void objects_library_embeds_the_source_a_language_server_opens()
        {
            using var archive = ZipFile.OpenRead(packed.ObjectsPackage);
            var entry = archive.Entries.Single(candidate => candidate.FullName.StartsWith("lib/", StringComparison.Ordinal) && candidate.FullName.EndsWith("/Moonwell.Objects.dll", StringComparison.Ordinal));
            using var library = new MemoryStream();
            using (var stream = entry.Open())
            {
                stream.CopyTo(library);
            }

            library.Position = 0;
            using var pe = new PEReader(library);
            var pdb = Assert.Single(pe.ReadDebugDirectory(), candidate => candidate.Type == DebugDirectoryEntryType.EmbeddedPortablePdb);
            using var provider = pe.ReadEmbeddedPortablePdbDebugDirectoryData(pdb);
            var reader = provider.GetMetadataReader();

            // A document's path is only a name (the text is embedded), and a released package must not carry this machine's folders.
            var names = reader.Documents.Select(handle => reader.GetString(reader.GetDocument(handle).Name)).ToList();
            Assert.NotEmpty(names);
            Assert.All(names, name =>
            {
                Assert.StartsWith("/_/", name, StringComparison.Ordinal);
                Assert.DoesNotContain(":", name);
                Assert.DoesNotContain("Users", name);
            });

            // Go to Definition on Paladin opens this file's text from the library: its constructor sets the values.
            var document = reader.Documents
                .Select(handle => (Handle: (EntityHandle)handle, Name: reader.GetString(reader.GetDocument(handle).Name)))
                .Single(candidate => candidate.Name.Replace('\\', '/').EndsWith("/Generated/Human/Units.g.cs", StringComparison.Ordinal));
            var source = EmbeddedSource(reader, document.Handle);
            Assert.NotNull(source);
            Assert.Contains("public Paladin()", source);
            Assert.Contains("AcquisitionRange = 500f;", source);
        }

        // The text a portable PDB embeds for a document, or null when it embeds none (the format: Microsoft's PortablePdb-Metadata.md, "Embedded Source").
        private static string? EmbeddedSource(MetadataReader reader, EntityHandle document)
        {
            var embeddedSource = new Guid("0E8A571B-6926-466E-B4AD-8AB04611F5FE");
            foreach (var handle in reader.GetCustomDebugInformation(document))
            {
                var information = reader.GetCustomDebugInformation(handle);
                if (reader.GetGuid(information.Kind) != embeddedSource)
                {
                    continue;
                }

                var blob = reader.GetBlobBytes(information.Value);
                var uncompressedLength = BitConverter.ToInt32(blob, 0);
                if (uncompressedLength == 0)
                {
                    return System.Text.Encoding.UTF8.GetString(blob, 4, blob.Length - 4);
                }

                using var deflate = new DeflateStream(new MemoryStream(blob, 4, blob.Length - 4), CompressionMode.Decompress);
                using var text = new StreamReader(deflate, System.Text.Encoding.UTF8);
                return text.ReadToEnd();
            }

            return null;
        }

        [Fact]
        public void damage_package_has_source_and_depends_on_moonwell()
        {
            using var archive = ZipFile.OpenRead(packed.DamagePackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Damage/Hit.cs", names);
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Damage.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Damage.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var files = nuspec.Descendants().Where(element => element.Name.LocalName == "files").ToList();
            var hit = files.Single(file => file.Attribute("include")!.Value.Replace('\\', '/') == "cs/net10.0/Moonwell.Damage/Hit.cs");
            Assert.Equal("None", hit.Attribute("buildAction")!.Value);
            var dependency = Assert.Single(nuspec.Descendants(), element => element.Name.LocalName == "dependency" && element.Attribute("id")!.Value == "Moonwell");
            Assert.Equal(packed.Version, dependency.Attribute("version")!.Value);
        }

        [Fact]
        public void maths_package_carries_its_source()
        {
            using var archive = ZipFile.OpenRead(packed.MathsPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Maths/Vector2.cs", names);
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Maths.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Maths.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var dependency = Assert.Single(nuspec.Descendants(), element => element.Name.LocalName == "dependency" && element.Attribute("id")!.Value == "Moonwell");
            Assert.Equal(packed.Version, dependency.Attribute("version")!.Value);
        }

        [Fact]
        public void interactions_package_carries_its_source()
        {
            using var archive = ZipFile.OpenRead(packed.InteractionsPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Interactions/Gizmo.cs", names);
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Interactions.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Interactions.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var dependencies = nuspec.Descendants().Where(element => element.Name.LocalName == "dependency").ToList();
            var moonwell = Assert.Single(dependencies, element => element.Attribute("id")!.Value == "Moonwell");
            Assert.Equal(packed.Version, moonwell.Attribute("version")!.Value);
            var maths = Assert.Single(dependencies, element => element.Attribute("id")!.Value == "Moonwell.Maths");
            Assert.Equal(packed.Version, maths.Attribute("version")!.Value);
        }

        [Fact]
        public void terrain_package_carries_its_source()
        {
            using var archive = ZipFile.OpenRead(packed.TerrainPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Terrain/Terrain.cs", names);
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Terrain.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Terrain.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var dependency = Assert.Single(nuspec.Descendants(), element => element.Name.LocalName == "dependency" && element.Attribute("id")!.Value == "Moonwell");
            Assert.Equal(packed.Version, dependency.Attribute("version")!.Value);
        }

        [Fact]
        public void physics_package_carries_its_source()
        {
            using var archive = ZipFile.OpenRead(packed.PhysicsPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Physics/Missile.cs", names);
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Physics.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Physics.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var dependencies = nuspec.Descendants().Where(element => element.Name.LocalName == "dependency").ToList();
            foreach (var id in new[] { "Moonwell", "Moonwell.Maths", "Moonwell.Terrain", "Moonwell.Interactions" })
            {
                var dependency = Assert.Single(dependencies, element => element.Attribute("id")!.Value == id);
                Assert.Equal(packed.Version, dependency.Attribute("version")!.Value);
            }
        }

        [Fact]
        public void areas_package_carries_its_source()
        {
            using var archive = ZipFile.OpenRead(packed.AreasPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Areas/Area.cs", names);
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Areas.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Areas.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var dependencies = nuspec.Descendants().Where(element => element.Name.LocalName == "dependency").ToList();
            var moonwell = Assert.Single(dependencies, element => element.Attribute("id")!.Value == "Moonwell");
            Assert.Equal(packed.Version, moonwell.Attribute("version")!.Value);
            var maths = Assert.Single(dependencies, element => element.Attribute("id")!.Value == "Moonwell.Maths");
            Assert.Equal(packed.Version, maths.Attribute("version")!.Value);
        }

        [Fact]
        public void input_package_carries_its_source()
        {
            using var archive = ZipFile.OpenRead(packed.InputPackage);
            var names = archive.Entries.Select(entry => entry.FullName).ToList();
            Assert.Contains("contentFiles/cs/net10.0/Moonwell.Input/Mouse.cs", names);
            Assert.Contains(names, name => name.StartsWith("lib/", StringComparison.Ordinal) && name.EndsWith("/Moonwell.Input.dll", StringComparison.Ordinal));
            Assert.DoesNotContain(names, name => name.Contains("/obj/") || name.Contains("/bin/"));

            using var reader = new StreamReader(archive.Entries.Single(entry => entry.FullName == "Moonwell.Input.nuspec").Open());
            var nuspec = XDocument.Parse(reader.ReadToEnd());
            var dependencies = nuspec.Descendants().Where(element => element.Name.LocalName == "dependency").ToList();
            var moonwell = Assert.Single(dependencies, element => element.Attribute("id")!.Value == "Moonwell");
            Assert.Equal(packed.Version, moonwell.Attribute("version")!.Value);
            var maths = Assert.Single(dependencies, element => element.Attribute("id")!.Value == "Moonwell.Maths");
            Assert.Equal(packed.Version, maths.Attribute("version")!.Value);
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
        public void template_map_has_the_game_data_the_build_writes()
        {
            var info = Path.Combine(TestFolders.Fixture("."), "..", "..", "..", "template", "MoonwellMap", "map", "war3map.w3i");
            using var reader = new BinaryReader(File.OpenRead(info));

            var data = reader.ReadMapInfo();

            Assert.Equal(GameDataSet.Default, data.GameDataSet);
            Assert.Equal(GameDataVersion.ForsakenKingdom, data.GameDataVersion);
        }

        [Fact]
        public void template_map_builds()
        {
            var hive = folders.TemporaryFolderOutsideRepository();
            var parent = folders.TemporaryFolderOutsideRepository();
            PackedMoonwell.Dotnet(parent, "new", "install", packed.TemplatesPackage, "--debug:custom-hive", hive);
            PackedMoonwell.Dotnet(parent, "new", "moonwell", "-n", "Keep", "--debug:custom-hive", hive);
            var map = Path.Combine(parent, "Keep");
            WriteFeedConfig(map);

            // The map's own code is the template's, so a failure shows the build's output.
            var log = new StringWriter();
            string archive;
            try
            {
                archive = MapBuild.Run(map, new BuildOptions(), log);
            }
            catch (BuildException failure)
            {
                throw new Xunit.Sdk.XunitException(failure.Message + "\n" + log);
            }

            Assert.Equal(Path.Combine(map, "build", "Keep.w3x"), archive);
            Assert.True(File.Exists(archive), log.ToString());
            Assert.DoesNotContain("note:", log.ToString());
            Assert.Contains("Name = \"Keep\";", File.ReadAllText(Path.Combine(map, "Map.cs")));
            var script = File.ReadAllText(Path.Combine(map, "build", "Keep", "war3map.lua"));
            Assert.Contains("[\"Keep.Heroes.Champion\"]", script);

            // The Champion is a Paladin whose MaxLife = MaxLife + 200 starts him at 850: his base field is that less his 22 strength's 550.
            using var reader = new BinaryReader(File.OpenRead(Path.Combine(map, "build", "Keep", "war3map.w3u")), System.Text.Encoding.UTF8);
            var champion = Assert.Single(reader.ReadUnitObjectData().NewUnits);
            Assert.Equal(300, Assert.Single(champion.Modifications, one => one.Id == BitConverter.ToInt32(System.Text.Encoding.ASCII.GetBytes("uhpm"), 0)).Value);
        }

        [Fact]
        public void type_values_are_written_only_when_the_map_reads_default()
        {
            // The template's code never reads Default: its release script has no table by type id (the runtime's own Lua names it).
            var hive = folders.TemporaryFolderOutsideRepository();
            var parent = folders.TemporaryFolderOutsideRepository();
            PackedMoonwell.Dotnet(parent, "new", "install", packed.TemplatesPackage, "--debug:custom-hive", hive);
            PackedMoonwell.Dotnet(parent, "new", "moonwell", "-n", "Keep", "--debug:custom-hive", hive);
            var map = Path.Combine(parent, "Keep");
            WriteFeedConfig(map);

            MapBuild.Run(map, new BuildOptions { Release = true }, new StringWriter());

            var script = File.ReadAllText(Path.Combine(map, "build", "Keep", "war3map.lua"));
            Assert.Contains("MoonwellObjectValues = {", script);
            Assert.DoesNotContain("MoonwellTypeValues = {", script);
        }

        [Fact]
        public void template_map_calls_no_native()
        {
            var template = Path.Combine(TestFolders.Fixture("."), "..", "..", "..", "template", "MoonwellMap");
            var calls = Directory.GetFiles(template, "*.cs", SearchOption.AllDirectories)
                .Where(file => !file.Contains(Path.DirectorySeparatorChar + "obj" + Path.DirectorySeparatorChar) && !file.Contains(Path.DirectorySeparatorChar + "bin" + Path.DirectorySeparatorChar))
                .SelectMany(file => Regex.Matches(File.ReadAllText(file), @"Natives\.\w+").Select(match => match.Value))
                .ToList();

            // Smite reads the target's name as Unit.Name inside Local.For; the template calls no native at all.
            Assert.Empty(calls);
        }

        [Fact]
        public void template_map_shows_adding_the_damage_package_and_builds_with_it()
        {
            var hive = folders.TemporaryFolderOutsideRepository();
            var parent = folders.TemporaryFolderOutsideRepository();
            PackedMoonwell.Dotnet(parent, "new", "install", packed.TemplatesPackage, "--debug:custom-hive", hive);
            PackedMoonwell.Dotnet(parent, "new", "moonwell", "-n", "Keep", "--debug:custom-hive", hive);
            var map = Path.Combine(parent, "Keep");
            WriteFeedConfig(map);

            // The guide's one line: take the comment off the project's Moonwell.Damage line.
            var csproj = Path.Combine(map, "Keep.csproj");
            var text = File.ReadAllText(csproj);
            var uncommented = Regex.Replace(text, @"<!--\s*(<PackageReference Include=""Moonwell\.Damage"" [^>]*/>)\s*-->", "$1");
            Assert.NotEqual(text, uncommented);
            Assert.Contains("<PackageReference Include=\"Moonwell.Damage\" Version=\"" + packed.Version + "\" />", uncommented);
            File.WriteAllText(csproj, uncommented);

            // The build starts the package once the map's code uses one of its types, as the guide's Shield does.
            File.WriteAllText(
                Path.Combine(map, "Shield.cs"),
                "namespace Keep\n{\n    public sealed class Shield : Moonwell.Buff, Moonwell.IDamageTarget\n    {\n        public void OnTaken(Moonwell.Hit hit) { hit.Amount *= 0.5f; }\n    }\n}\n");

            var log = new StringWriter();
            string archive;
            try
            {
                archive = MapBuild.Run(map, new BuildOptions(), log);
            }
            catch (BuildException failure)
            {
                throw new Xunit.Sdk.XunitException(failure.Message + "\n" + log);
            }

            Assert.True(File.Exists(archive), log.ToString());
            var script = File.ReadAllText(Path.Combine(map, "build", "Keep", "war3map.lua"));
            Assert.Contains("DamageSystem.Install()", script);
        }

        [Fact]
        public void template_map_release_carries_no_damage_package()
        {
            var hive = folders.TemporaryFolderOutsideRepository();
            var parent = folders.TemporaryFolderOutsideRepository();
            PackedMoonwell.Dotnet(parent, "new", "install", packed.TemplatesPackage, "--debug:custom-hive", hive);
            PackedMoonwell.Dotnet(parent, "new", "moonwell", "-n", "Keep", "--debug:custom-hive", hive);
            var map = Path.Combine(parent, "Keep");
            WriteFeedConfig(map);

            var log = new StringWriter();
            MapBuild.Run(map, new BuildOptions { Release = true }, log);

            var script = File.ReadAllText(Path.Combine(map, "build", "Keep", "war3map.lua"));
            Assert.DoesNotContain("DamageSystem", script);
        }

        [Fact]
        public void package_without_source_is_reported()
        {
            var copy = folders.TemporaryCopyOfSample();
            WriteAssets(copy, "Moonwell/0.0.0-missing", "package");

            var error = Assert.Throws<BuildException>(() => PackageSources.Check(MapProject.Find(copy)));

            var packages = CSharpLua.PackageHelper.GetGlobalPackagesPath();
            Assert.Equal($"Moonwell 0.0.0-missing has no source in {packages}, where CSharp.lua reads packages from", error.Message);
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
            }

            if (withoutOwnRemove)
            {
                var removed = Regex.Replace(replaced, @"[ \t]*<Compile Remove=""build/\*\*"" />\r?\n", string.Empty);
                Assert.NotEqual(replaced, removed);
                replaced = removed;
            }

            File.WriteAllText(project, replaced);
            WriteFeedConfig(copy);
            return copy;
        }

        // Restores the folder's project from the test feed only.
        private void WriteFeedConfig(string folder)
        {
            File.WriteAllText(
                Path.Combine(folder, "nuget.config"),
                "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"
                + "<configuration>\n"
                + "  <packageSources>\n"
                + "    <clear />\n"
                + $"    <add key=\"moonwell-test\" value=\"{packed.Feed}\" />\n"
                + "  </packageSources>\n"
                + "</configuration>\n");
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
