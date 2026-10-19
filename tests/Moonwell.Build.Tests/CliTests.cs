using System.Text.RegularExpressions;
using Moonwell.Tool;
using War3Net.IO.Mpq;
using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class CliTests : IDisposable
    {
        private const string Usage =
            "usage: moonwell build [folder] [--release]\n" +
            "       moonwell run [folder] [--release] [--game <exe>]";

        private readonly SampleBuilds builds;
        private readonly TestFolders folders = new TestFolders();
        private readonly List<string> temporaryFiles = new List<string>();

        public CliTests(SampleBuilds builds)
        {
            this.builds = builds;
        }

        [Fact]
        public void build_writes_archive()
        {
            Assert.Equal(0, builds.Development.ExitCode);
            Assert.Equal(0, builds.Release.ExitCode);
            Assert.EndsWith(@"moonwell: built build\Sample.w3x", builds.Development.Output.TrimEnd());
            Assert.EndsWith(@"moonwell: built build\Sample.w3x", builds.Release.Output.TrimEnd());
            Assert.StartsWith("moonwell: Sample (development build)", builds.Development.Output);
            Assert.StartsWith("moonwell: Sample (release build)", builds.Release.Output);
            Assert.True(File.Exists(builds.Development.Archive));
            Assert.True(File.Exists(builds.Release.Archive));

            var development = ReadScript(builds.Development.Archive);
            var release = ReadScript(builds.Release.Archive);
            Assert.Contains("sample: started (development)", development);
            Assert.DoesNotContain("sample: started (release)", development);
            Assert.Contains("sample: started (release)", release);
            Assert.DoesNotContain("sample: started (development)", release);
        }

        [Fact]
        public void build_error_exits_1()
        {
            var (exitCode, _, error, _) = Run("build", TestFolders.Fixture("NoMap"));

            Assert.Equal(1, exitCode);
            Assert.Equal(@"moonwell: no World Editor map: map\war3map.w3i is missing", error.Trim());
        }

        [Fact]
        public void unexpected_error_exits_1()
        {
            var game = Path.Combine(Path.GetTempPath(), "moonwell-game-" + Guid.NewGuid().ToString("N") + ".exe");
            File.WriteAllText(game, string.Empty);
            temporaryFiles.Add(game);
            var output = new StringWriter();
            var error = new StringWriter();

            var exitCode = Cli.Run(
                new[] { "run", TestFolders.Fixture("Sample"), "--game", game },
                output,
                error,
                (_, _) => throw new InvalidOperationException("starter failed"));

            Assert.Equal(1, exitCode);
            Assert.StartsWith("moonwell: unexpected error: InvalidOperationException: starter failed", error.ToString());
            Assert.Contains(" at ", error.ToString());
        }

        [Fact]
        public void locked_archive_is_reported()
        {
            var folder = folders.TemporaryCopyOfSample();
            var archive = Path.Combine(folder, "build", "Sample.w3x");
            Directory.CreateDirectory(Path.GetDirectoryName(archive)!);

            int exitCode;
            string error;
            using (new FileStream(archive, FileMode.Create, FileAccess.ReadWrite, FileShare.None))
            {
                (exitCode, _, error, _) = Run("build", folder);
            }

            Assert.Equal(1, exitCode);
            Assert.StartsWith(@"moonwell: cannot write build\Sample.w3x", error);
            Assert.Contains("(is the game or World Editor using it?)", error);
        }

        [Theory]
        [InlineData]
        [InlineData("deploy")]
        [InlineData("build", "--fast")]
        [InlineData("build", "one", "two")]
        [InlineData("build", "--game", "game.exe")]
        [InlineData("run", "--game")]
        public void bad_arguments_exit_2(params string[] args)
        {
            var (exitCode, output, error, started) = Run(args);

            Assert.Equal(2, exitCode);
            Assert.Equal(string.Empty, output);
            Assert.Equal(Usage, error.Trim());
            Assert.Empty(started);
        }

        [Fact]
        public void version_prints_the_tools_version()
        {
            var props = File.ReadAllText(Path.Combine(TestFolders.Fixture("."), "..", "..", "..", "Directory.Build.props"));
            var version = Regex.Match(props, "<Version>([^<]+)</Version>").Groups[1].Value;

            var (exitCode, output, error, started) = Run("--version");

            Assert.Equal(0, exitCode);
            Assert.Equal(version, output.Trim());
            Assert.Equal(string.Empty, error);
            Assert.Empty(started);
        }

        [Fact]
        public void run_starts_the_game()
        {
            var game = Path.Combine(Path.GetTempPath(), "moonwell-game-" + Guid.NewGuid().ToString("N") + ".exe");
            File.WriteAllText(game, string.Empty);
            temporaryFiles.Add(game);
            var sample = TestFolders.Fixture("Sample");

            var (exitCode, _, _, started) = Run("run", sample, "--game", game);

            Assert.Equal(0, exitCode);
            var call = Assert.Single(started);
            Assert.Equal(game, call.Exe);
            var archive = Path.Combine(sample, "build", "Sample.w3x");
            Assert.Equal($"-launch -windowmode windowed -loadfile \"{archive}\"", call.Arguments);
        }

        [Fact]
        public void run_without_game_exits_1()
        {
            var game = Path.Combine(Path.GetTempPath(), "moonwell-missing-" + Guid.NewGuid().ToString("N") + ".exe");

            var (exitCode, output, error, started) = Run("run", TestFolders.Fixture("Sample"), "--game", game);

            Assert.Equal(1, exitCode);
            Assert.Equal($"moonwell: the game is not at {game}", error.Trim());
            Assert.Empty(started);
            Assert.Equal(string.Empty, output);
        }

        public void Dispose()
        {
            folders.Dispose();
            foreach (var file in temporaryFiles)
            {
                File.Delete(file);
            }
        }

        private static (int ExitCode, string Output, string Error, List<(string Exe, string Arguments)> Started) Run(params string[] args)
        {
            var output = new StringWriter();
            var error = new StringWriter();
            var started = new List<(string Exe, string Arguments)>();
            var exitCode = Cli.Run(args, output, error, (exe, arguments) => started.Add((exe, arguments)));
            return (exitCode, output.ToString(), error.ToString(), started);
        }

        private static string ReadScript(string archivePath)
        {
            using var archive = MpqArchive.Open(archivePath, true);
            using var stream = archive.OpenFile("war3map.lua");
            using var reader = new StreamReader(stream);
            return reader.ReadToEnd();
        }
    }
}
