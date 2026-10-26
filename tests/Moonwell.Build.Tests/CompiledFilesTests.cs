using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>The files the build reads as the map's code: the stage and the stubs' look at the source agree.</summary>
    [Collection("Sample")]
    public sealed class CompiledFilesTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders;

        public CompiledFilesTests(SampleBuilds builds, ITestOutputHelper output)
        {
            this.builds = builds;
            folders = new TestFolders(output);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void sources_leave_out_the_top_level_bin_and_obj_and_the_folders_given()
        {
            var folder = folders.TemporaryFolderOutsideRepository();
            foreach (var file in new[] { "A.cs", "bin/B.cs", "obj/C.cs", "Sub/D.cs", "Sub/obj/G.cs", "build/E.cs", "build/marked/F.cs" })
            {
                Write(folder, file, "class X { }");
            }

            Assert.Equal(
                new[] { "A.cs", "Sub/D.cs", "Sub/obj/G.cs", "build/E.cs", "build/marked/F.cs" },
                Relative(folder, CompiledFiles.Sources(folder)));
            Assert.Equal(new[] { "A.cs", "Sub/D.cs", "Sub/obj/G.cs" }, Relative(folder, CompiledFiles.Sources(folder, Path.Combine(folder, "build"))));
            Assert.Equal(
                new[] { "A.cs", "Sub/D.cs", "Sub/obj/G.cs", "build/E.cs" },
                Relative(folder, CompiledFiles.Sources(folder, Path.Combine(folder, "build", "marked"))));
        }

        [Fact]
        public void the_marked_copy_does_not_read_a_file_under_bin_or_obj()
        {
            var copy = folders.TemporaryCopyOfSample();
            Write(copy, "bin/Junk.cs", "class Junk { }");
            Write(copy, "obj/Junk.cs", "class Junk { }");

            var marked = MarkedProject.Stage(MapProject.Find(copy), new[] { "MOONWELL_LUA" });

            Assert.DoesNotContain(marked.Files, file => file.StartsWith("bin/", StringComparison.Ordinal) || file.StartsWith("obj/", StringComparison.Ordinal));
            Assert.Contains("Objects.cs", marked.Files);
        }

        [Fact]
        public void the_stubs_do_not_read_a_file_under_bin_or_obj_or_build()
        {
            var copy = folders.TemporaryCopyOfSample();
            Write(copy, "Names.cs", "namespace Sample { public static class Names { public const int Magic = (int)Moonwell.Objects.Targets.Air; } }");
            foreach (var junk in new[] { "bin/Junk.cs", "obj/Junk.cs", "build/Junk.cs" })
            {
                Write(copy, junk, "namespace Sample { public static class " + junk.Substring(0, junk.IndexOf('/')) + "Names { public const int Magic = (int)Moonwell.Objects.ArmorType.Metal; } }");
            }

            // The Sample's own output, which the collection's build makes.
            Assert.Equal(0, builds.Development.ExitCode);
            var assembly = Path.Combine(builds.Folder, "bin", "Debug", "net10.0", "Sample.dll");
            var found = NamedChoices.Read(MapProject.Find(copy), assembly, new HashSet<string> { "Targets", "ArmorType" });

            Assert.Equal(new[] { "Targets" }, found);
        }

        private static void Write(string folder, string relative, string text)
        {
            var path = Path.Combine(folder, relative);
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllText(path, text);
        }

        private static List<string> Relative(string folder, IEnumerable<string> files)
        {
            return files.Select(file => Path.GetRelativePath(folder, file).Replace(Path.DirectorySeparatorChar, '/')).OrderBy(file => file, StringComparer.Ordinal).ToList();
        }
    }
}
