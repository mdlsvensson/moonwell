using System.Text.Json;

namespace Moonwell.Build
{
    /// <summary>Checks that the Moonwell packages a map project uses carry their source in the folder CSharp.lua reads packages from.</summary>
    public static class PackageSources
    {
        /// <summary>Checks every <c>Moonwell</c> and <c>Moonwell.*</c> package the project restored.</summary>
        /// <param name="project">The map project, whose <c>obj\project.assets.json</c> <c>dotnet build</c> has just written.</param>
        /// <exception cref="BuildException">A package has no C# source in the folder NuGet restores packages to.</exception>
        public static void Check(MapProject project)
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

                var source = Path.Combine(defaultFolder, id.ToLowerInvariant(), version.ToLowerInvariant(), "contentFiles", "cs");
                if (!Directory.Exists(source))
                {
                    throw new BuildException($"{id} {version} has no source in {defaultFolder}, where CSharp.lua reads packages from");
                }
            }
        }
    }
}
