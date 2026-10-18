using System.Globalization;
using System.Reflection;
using System.Runtime.CompilerServices;

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
                    throw new BuildException($"{MapClasses.Describe(type)}.Main: a map declares no Main; the build writes it");
                }
            }

            var maps = types.Where(IsMapClass).ToList();
            if (maps.Count == 0)
            {
                throw new BuildException($"no map class: declare one class that derives from {MapBaseName}");
            }

            if (maps.Count > 1)
            {
                throw new BuildException("more than one map class: " + string.Join(", ", maps.Select(MapClasses.Describe)));
            }

            var mapType = maps[0];
            var name = MapClasses.Describe(mapType);
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
                throw MapClasses.ConstructionError(wrapped.InnerException, name, project);
            }

            return new MapSettings(
                name,
                ReadSetting(map, "Name"),
                ReadSetting(map, "Author"),
                ReadSetting(map, "Description"),
                ReadSetting(map, "RecommendedPlayers"),
                ReadNumber(map, "DataSet"),
                ReadNumber(map, "DataVersion"));
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
            return (string?)SettingOf(map, property).GetValue(map) ?? string.Empty;
        }

        // The enum crosses the load context as its number.
        private static int ReadNumber(object map, string property)
        {
            return Convert.ToInt32(SettingOf(map, property).GetValue(map), CultureInfo.InvariantCulture);
        }

        // Read from the Moonwell.Map base type, so that a property the class hides with new is not asked for.
        private static PropertyInfo SettingOf(object map, string property)
        {
            var baseType = map.GetType().BaseType!;
            while (baseType.FullName != MapBaseName)
            {
                baseType = baseType.BaseType!;
            }

            return baseType.GetProperty(property, BindingFlags.Instance | BindingFlags.Public)!;
        }
    }
}
