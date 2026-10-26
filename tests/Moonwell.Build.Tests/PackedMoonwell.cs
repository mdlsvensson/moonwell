using System.Diagnostics;

namespace Moonwell.Build.Tests
{
    /// <summary>Moonwell, Moonwell.Natives, Moonwell.Objects, Moonwell.Damage, Moonwell.Maths, Moonwell.Interactions, Moonwell.Terrain, Moonwell.Areas, Moonwell.Input and Moonwell.Templates packed with a version of their own into a feed in the temporary folder; removes everything it made, including what a restore left in the NuGet folder.</summary>
    public sealed class PackedMoonwell : IDisposable
    {
        private readonly string root = Path.Combine(Path.GetTempPath(), "moonwell-feed-" + Guid.NewGuid().ToString("N"));

        /// <summary>Packs the nine runtime projects and the template.</summary>
        public PackedMoonwell()
        {
            Version = "0.1.0-test." + DateTime.UtcNow.Ticks;
            Feed = Path.Combine(root, "feed");
            Directory.CreateDirectory(Feed);

            var repository = Path.GetFullPath(Path.Combine(TestFolders.Fixture("."), "..", "..", ".."));
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Natives", "Moonwell.Natives.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell", "Moonwell.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Objects", "Moonwell.Objects.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Damage", "Moonwell.Damage.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Maths", "Moonwell.Maths.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Interactions", "Moonwell.Interactions.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Terrain", "Moonwell.Terrain.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Areas", "Moonwell.Areas.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("src", "Moonwell.Input", "Moonwell.Input.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
            Dotnet(repository, "pack", Path.Combine("template", "Moonwell.Templates.csproj"), "-c", "Release", "-o", Feed, "-p:Version=" + Version, "--disable-build-servers");
        }

        /// <summary>The version all ten packages were packed with, <c>0.1.0-test.</c> and a number nothing else uses.</summary>
        public string Version { get; }

        /// <summary>The folder holding the ten <c>.nupkg</c> files.</summary>
        public string Feed { get; }

        /// <summary>The path of the Moonwell package.</summary>
        public string MoonwellPackage => Path.Combine(Feed, "Moonwell." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Objects package.</summary>
        public string ObjectsPackage => Path.Combine(Feed, "Moonwell.Objects." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Damage package.</summary>
        public string DamagePackage => Path.Combine(Feed, "Moonwell.Damage." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Maths package.</summary>
        public string MathsPackage => Path.Combine(Feed, "Moonwell.Maths." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Interactions package.</summary>
        public string InteractionsPackage => Path.Combine(Feed, "Moonwell.Interactions." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Terrain package.</summary>
        public string TerrainPackage => Path.Combine(Feed, "Moonwell.Terrain." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Areas package.</summary>
        public string AreasPackage => Path.Combine(Feed, "Moonwell.Areas." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Input package.</summary>
        public string InputPackage => Path.Combine(Feed, "Moonwell.Input." + Version + ".nupkg");

        /// <summary>The path of the Moonwell.Templates package.</summary>
        public string TemplatesPackage => Path.Combine(Feed, "Moonwell.Templates." + Version + ".nupkg");

        /// <summary>Deletes the feed and the packages the restore copied into the NuGet folder (see TestFolders.Delete for a folder still in use).</summary>
        public void Dispose()
        {
            var packages = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".nuget", "packages");
            foreach (var id in new[] { "moonwell", "moonwell.natives", "moonwell.objects", "moonwell.damage", "moonwell.maths", "moonwell.interactions", "moonwell.terrain", "moonwell.areas", "moonwell.input" })
            {
                TestFolders.Delete(Path.Combine(packages, id, Version.ToLowerInvariant()));

                var idFolder = Path.Combine(packages, id);
                if (Directory.Exists(idFolder) && !Directory.EnumerateFileSystemEntries(idFolder).Any())
                {
                    TestFolders.Delete(idFolder, false);
                }
            }

            TestFolders.Delete(root);
        }

        /// <summary>Runs dotnet in a folder, and fails with its output when it fails.</summary>
        public static void Dotnet(string folder, params string[] arguments)
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
