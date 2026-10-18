using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>Defines the collection of tests that build the Sample, and gives it the Sample's builds, made once.</summary>
    [CollectionDefinition("Sample")]
    public sealed class SampleCollection : ICollectionFixture<SampleBuilds>
    {
    }

    /// <summary>The Sample built through the command, as a development and as a release build, copied out of the Sample's build folder.</summary>
    public sealed class SampleBuilds : IDisposable
    {
        private readonly string root = Path.Combine(Path.GetTempPath(), "moonwell-samplebuilds-" + Guid.NewGuid().ToString("N"));

        /// <summary>Builds the Sample both ways.</summary>
        public SampleBuilds()
        {
            var sample = TestFolders.Fixture("Sample");
            Development = Build(sample, "Development", false);
            Release = Build(sample, "Release", true);
        }

        /// <summary>The development build.</summary>
        public BuiltSample Development { get; }

        /// <summary>The release build.</summary>
        public BuiltSample Release { get; }

        /// <summary>Deletes the copies.</summary>
        public void Dispose()
        {
            if (Directory.Exists(root))
            {
                Directory.Delete(root, true);
            }
        }

        private BuiltSample Build(string sample, string name, bool release)
        {
            var output = new StringWriter();
            var error = new StringWriter();
            var arguments = release ? new[] { "build", sample, "--release" } : new[] { "build", sample };
            var exitCode = Cli.Run(arguments, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            var folder = Path.Combine(root, name);
            CopyFolder(Path.Combine(sample, "build"), folder);
            return new BuiltSample(exitCode, output.ToString(), error.ToString(), folder);
        }

        private static void CopyFolder(string from, string to)
        {
            Directory.CreateDirectory(to);
            if (!Directory.Exists(from))
            {
                return;
            }

            foreach (var file in Directory.GetFiles(from))
            {
                File.Copy(file, Path.Combine(to, Path.GetFileName(file)));
            }

            foreach (var directory in Directory.GetDirectories(from))
            {
                CopyFolder(directory, Path.Combine(to, Path.GetFileName(directory)));
            }
        }
    }

    /// <summary>One build of the Sample: what the command returned and printed, and a copy of what it left in <c>build</c>.</summary>
    /// <param name="ExitCode">The command's exit code.</param>
    /// <param name="Output">What the command printed on its output.</param>
    /// <param name="Error">What the command printed on its error.</param>
    /// <param name="BuildFolder">The copy of the Sample's <c>build</c> folder.</param>
    public sealed record BuiltSample(int ExitCode, string Output, string Error, string BuildFolder)
    {
        /// <summary>The map archive.</summary>
        public string Archive => Path.Combine(BuildFolder, "Sample.w3x");

        /// <summary>The staged <c>war3map.lua</c>.</summary>
        public string Script => File.ReadAllText(Path.Combine(BuildFolder, "Sample", "war3map.lua"));
    }
}
