using System.Diagnostics;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Runtime.Loader;

namespace Moonwell.Build
{
    /// <summary>Reads the map's settings by constructing its <c>Moonwell.Map</c> class on .NET.</summary>
    public static class MapSettingsReader
    {
        private const string MapBaseName = "Moonwell.Map";

        /// <summary>Loads the built assembly, finds the map class, constructs it and reads its settings.</summary>
        /// <param name="assemblyPath">The map's built assembly.</param>
        /// <param name="project">The map project; lines in messages are relative to its folder.</param>
        /// <returns>The map's settings.</returns>
        /// <exception cref="BuildException">The assembly declares a Main, has no or several map classes, or the class does not construct.</exception>
        public static MapSettings Read(string assemblyPath, MapProject project)
        {
            if (!File.Exists(assemblyPath))
            {
                throw new BuildException($"the built assembly does not exist: {assemblyPath}");
            }

            var context = new MapLoadContext(Path.GetDirectoryName(Path.GetFullPath(assemblyPath))!);
            try
            {
                return ReadIn(context, assemblyPath, project);
            }
            finally
            {
                context.Unload();
            }
        }

        // Kept apart from Read so that nothing of the loaded assembly is on Read's frame when the context unloads.
        [MethodImpl(MethodImplOptions.NoInlining)]
        private static MapSettings ReadIn(MapLoadContext context, string assemblyPath, MapProject project)
        {
            var assembly = context.Open(assemblyPath);
            var types = assembly.GetTypes().OrderBy(type => type.FullName, StringComparer.Ordinal).ToList();

            foreach (var type in types)
            {
                var methods = type.GetMethods(BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.DeclaredOnly);
                if (methods.Any(method => method.Name == "Main"))
                {
                    throw new BuildException($"{Describe(type)}.Main: a map declares no Main; the build writes it");
                }
            }

            var maps = types.Where(IsMapClass).ToList();
            if (maps.Count == 0)
            {
                throw new BuildException($"no map class: declare one class that derives from {MapBaseName}");
            }

            if (maps.Count > 1)
            {
                throw new BuildException("more than one map class: " + string.Join(", ", maps.Select(Describe)));
            }

            var mapType = maps[0];
            var name = Describe(mapType);
            if (mapType.GetConstructor(Type.EmptyTypes) == null)
            {
                throw new BuildException($"{name} needs a public constructor without parameters");
            }

            object map;
            try
            {
                map = Activator.CreateInstance(mapType)!;
            }
            catch (TargetInvocationException wrapped) when (wrapped.InnerException != null)
            {
                throw ConstructionError(wrapped.InnerException, name, project);
            }

            return new MapSettings(
                name,
                ReadSetting(map, "Name"),
                ReadSetting(map, "Author"),
                ReadSetting(map, "Description"),
                ReadSetting(map, "RecommendedPlayers"));
        }

        private static string Describe(Type type)
        {
            return type.FullName!.Replace('+', '.');
        }

        private static bool IsMapClass(Type type)
        {
            if (type.IsAbstract)
            {
                return false;
            }

            for (var parent = type.BaseType; parent != null; parent = parent.BaseType)
            {
                if (parent.FullName == MapBaseName)
                {
                    return true;
                }
            }

            return false;
        }

        private static string ReadSetting(object map, string property)
        {
            // Read from the Moonwell.Map base type, so that a property the class hides with new is not asked for.
            var baseType = map.GetType().BaseType!;
            while (baseType.FullName != MapBaseName)
            {
                baseType = baseType.BaseType!;
            }

            var info = baseType.GetProperty(property, BindingFlags.Instance | BindingFlags.Public)!;
            return (string?)info.GetValue(map) ?? string.Empty;
        }

        private static BuildException ConstructionError(Exception inner, string name, MapProject project)
        {
            // A static field or a static constructor that throws surfaces as the type initializer's failure.
            while (inner is TypeInitializationException { InnerException: not null })
            {
                inner = inner.InnerException!;
            }

            var place = FirstLineInProject(inner, project);
            var prefix = place == null ? string.Empty : place + ": ";
            if (inner is BadImageFormatException)
            {
                return new BuildException($"{prefix}{name} calls the game while it is declared; game work starts from a reaction or the game's start");
            }

            return new BuildException($"{prefix}{name}'s constructor threw: {inner.Message}");
        }

        private static string? FirstLineInProject(Exception exception, MapProject project)
        {
            var root = project.Folder + Path.DirectorySeparatorChar;
            foreach (var frame in new StackTrace(exception, true).GetFrames())
            {
                var file = frame.GetFileName();
                if (file != null && file.StartsWith(root, StringComparison.OrdinalIgnoreCase))
                {
                    return $"{Path.GetRelativePath(project.Folder, file)}:{frame.GetFileLineNumber()}";
                }
            }

            return null;
        }

        // Loads from bytes, so that no file stays locked, and finds the map's own copies of Moonwell beside it.
        private sealed class MapLoadContext : AssemblyLoadContext
        {
            private readonly string folder;

            public MapLoadContext(string folder)
                : base("moonwell-map", true)
            {
                this.folder = folder;
            }

            public Assembly Open(string path)
            {
                return Read(path);
            }

            protected override Assembly? Load(AssemblyName assemblyName)
            {
                var path = Path.Combine(folder, assemblyName.Name + ".dll");
                return File.Exists(path) ? Read(path) : null;
            }

            private Assembly Read(string path)
            {
                using var assembly = new MemoryStream(File.ReadAllBytes(path));
                var symbols = Path.ChangeExtension(path, ".pdb");
                if (File.Exists(symbols))
                {
                    using var pdb = new MemoryStream(File.ReadAllBytes(symbols));
                    return LoadFromStream(assembly, pdb);
                }

                return LoadFromStream(assembly);
            }
        }
    }
}
