using System.Reflection;
using System.Text.Json;

namespace Moonwell.Build
{
    /// <summary>
    /// Checks that the Moonwell packages a map project uses are of this build's version and carry their source in the folder
    /// CSharp.lua reads packages from.
    /// </summary>
    public static class PackageSources
    {
        /// <summary>This build's version, as the <c>moonwell</c> tool and the packages released with it have it, without the source revision.</summary>
        public static string ToolVersion { get; } = OwnVersion();

        /// <summary>Checks every <c>Moonwell</c> and <c>Moonwell.*</c> package the project restored.</summary>
        /// <param name="project">The map project, whose <c>obj\project.assets.json</c> <c>dotnet build</c> has just written.</param>
        /// <param name="toolVersion">
        /// The version the packages must have: <see cref="ToolVersion"/>. The tool reads the internals of <c>Moonwell.Objects</c>
        /// and writes calls into the packages, so a package of another version fails in ways that do not say why.
        /// </param>
        /// <exception cref="BuildException">A package is of another version, or has no C# source in the folder NuGet restores packages to.</exception>
        public static void Check(MapProject project, string toolVersion)
        {
            var assets = Path.Combine(project.Folder, "obj", "project.assets.json");
            if (!File.Exists(assets))
            {
                return;
            }

            var defaultFolder = CSharpLua.PackageHelper.GetGlobalPackagesPath();
            using var stream = File.OpenRead(assets);
            using var document = JsonDocument.Parse(stream);
            if (!document.RootElement.TryGetProperty("libraries", out var libraries) || libraries.ValueKind != JsonValueKind.Object)
            {
                return;
            }

            foreach (var library in libraries.EnumerateObject())
            {
                var slash = library.Name.IndexOf('/');
                if (slash < 0)
                {
                    continue;
                }

                var id = library.Name.Substring(0, slash);
                var version = library.Name.Substring(slash + 1);
                var isPackage = library.Value.TryGetProperty("type", out var type) && type.GetString() == "package";
                var isMoonwell = id == "Moonwell" || id.StartsWith("Moonwell.", StringComparison.Ordinal);
                if (!isPackage || !isMoonwell)
                {
                    continue;
                }

                if (!string.Equals(version, toolVersion, StringComparison.OrdinalIgnoreCase))
                {
                    throw new BuildException(
                        $"the map's {id} is {version} and this moonwell is {toolVersion}, and they must be one version: run "
                        + $"dotnet tool update -g Moonwell.Cli --version {version}, or set the map's Moonwell PackageReference versions to {toolVersion}");
                }

                var source = Path.Combine(defaultFolder, id.ToLowerInvariant(), version.ToLowerInvariant(), "contentFiles", "cs");
                if (!Directory.Exists(source))
                {
                    throw new BuildException($"{id} {version} has no source in {defaultFolder}, where CSharp.lua reads packages from");
                }
            }
        }

        // The SDK appends the source revision after a plus sign.
        private static string OwnVersion()
        {
            var assembly = typeof(PackageSources).Assembly;
            var version = assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion
                ?? assembly.GetName().Version?.ToString(3)
                ?? string.Empty;
            var plus = version.IndexOf('+', StringComparison.Ordinal);
            return plus < 0 ? version : version.Substring(0, plus);
        }
    }
}
