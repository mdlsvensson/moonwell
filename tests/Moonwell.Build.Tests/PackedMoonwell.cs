using System.Diagnostics;

namespace Moonwell.Build.Tests
{
    /// <summary>Moonwell and Moonwell.Natives packed with a version of their own into a feed in the temporary folder; removes everything it made, including what a restore left in the NuGet folder.</summary>
    public sealed class PackedMoonwell : IDisposable
    {
        private readonly string root = Path.Combine(Path.GetTempPath(), "moonwell-feed-" + Guid.NewGuid().ToString("N"));

        /// <summary>Packs both runtime projects.</summary>
        public PackedMoonwell()
        {
            Version = "0.1.0-test." + DateTime.UtcNow.Ticks;
            Feed = Path.Combine(root, "feed");
            Directory.CreateDirectory(Feed);

            var repository = Path.GetFullPath(Path.Combine(TestFolders.Fixture("."), "..", "..", ".."));
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Natives", "Moonwell.Natives.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version);
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell", "Moonwell.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version);
        }

        /// <summary>The version both packages were packed with, <c>0.1.0-test.</c> and a number nothing else uses.</summary>
        public string Version { get; }

        /// <summary>The folder holding the two <c>.nupkg</c> files.</summary>
        public string Feed { get; }

        /// <summary>The path of the Moonwell package.</summary>
        public string MoonwellPackage => Path.Combine(Feed, "Moonwell." + Version + ".nupkg");

        /// <summary>Deletes the feed and the packages the restore copied into the NuGet folder.</summary>
        public void Dispose()
        {
            var packages = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".nuget", "packages");
            foreach (var id in new[] { "moonwell", "moonwell.natives" })
            {
                var version = Path.Combine(packages, id, Version.ToLowerInvariant());
                if (Directory.Exists(version))
                {
                    Directory.Delete(version, true);
                }

                var idFolder = Path.Combine(packages, id);
                if (Directory.Exists(idFolder) && !Directory.EnumerateFileSystemEntries(idFolder).Any())
                {
                    Directory.Delete(idFolder);
                }
            }

            if (Directory.Exists(root))
            {
                Directory.Delete(root, true);
            }
        }

        private static void Dotnet(string folder, params string[] arguments)
        {
            var start = new ProcessStartInfo("dotnet")
            {
                WorkingDirectory = folder,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            foreach (var argument in arguments)
            {
                start.ArgumentList.Add(argument);
            }

            start.ArgumentList.Add("--disable-build-servers");
            using var process = Process.Start(start)!;
            var output = process.StandardOutput.ReadToEndAsync();
            var error = process.StandardError.ReadToEndAsync();
            process.WaitForExit();
            if (process.ExitCode != 0)
            {
                throw new InvalidOperationException($"dotnet {string.Join(' ', arguments)} failed:\n{output.Result}\n{error.Result}");
            }
        }
    }
}
