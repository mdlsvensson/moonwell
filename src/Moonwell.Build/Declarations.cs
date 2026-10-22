using System.Reflection;
using System.Reflection.Metadata;
using System.Reflection.PortableExecutable;
using System.Runtime.CompilerServices;

namespace Moonwell.Build
{
    /// <summary>Reads the objects a map declares by constructing their classes on .NET.</summary>
    public static class Declarations
    {
        internal const string ObjectsAssemblyName = "Moonwell.Objects";
        private const string StandardAttributeName = "Moonwell.Objects.StandardAttribute";
        private const string ReplacesAttributeName = "Moonwell.Objects.ReplacesAttribute";
        private const string ObjectIdAttributeName = "Moonwell.Objects.ObjectIdAttribute";

        internal static readonly Dictionary<string, string> KindOfBase = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            ["Moonwell.Objects.StandardUnit"] = "unit",
            ["Moonwell.Objects.StandardHero"] = "hero",
            ["Moonwell.Objects.StandardItem"] = "item",
            ["Moonwell.Objects.StandardDestructable"] = "destructable",
            ["Moonwell.Objects.StandardDoodad"] = "doodad",
            ["Moonwell.Objects.StandardAbility"] = "ability",
            ["Moonwell.Objects.StandardBuff"] = "buff",
            ["Moonwell.Objects.StandardUpgrade"] = "upgrade",
        };

        /// <summary>Tells whether the built assembly references <c>Moonwell.Objects</c>, read from its metadata without loading it.</summary>
        /// <param name="assemblyPath">The map's built assembly.</param>
        /// <returns>True when it references the library.</returns>
        public static bool UsesObjects(string assemblyPath)
        {
            return References(assemblyPath, ObjectsAssemblyName);
        }

        /// <summary>Whether a built assembly references an assembly by name; the compiler keeps only the references the code uses.</summary>
        /// <param name="assemblyPath">The map's built assembly.</param>
        /// <param name="assemblyName">The name of the referenced assembly, such as <c>Moonwell.Damage</c>.</param>
        public static bool References(string assemblyPath, string assemblyName)
        {
            using var stream = File.OpenRead(assemblyPath);
            using var pe = new PEReader(stream);
            var metadata = pe.GetMetadataReader();
            foreach (var handle in metadata.AssemblyReferences)
            {
                if (metadata.GetString(metadata.GetAssemblyReference(handle).Name) == assemblyName)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>
        /// Checks the setters a map without <c>Moonwell.Objects</c> calls: a unit's Level has no live setter, and such a map has no
        /// class whose constructor may set it (<see cref="Read"/> checks a map that uses the library).
        /// </summary>
        /// <param name="assemblyPath">The map's built assembly.</param>
        /// <exception cref="BuildException">A method sets a unit's Level.</exception>
        public static void CheckSetters(string assemblyPath)
        {
            var context = new MapLoadContext(Path.GetDirectoryName(Path.GetFullPath(assemblyPath))!);
            try
            {
                CheckSettersIn(context, assemblyPath);
            }
            finally
            {
                context.Unload();
            }
        }

        // Kept apart from CheckSetters so that nothing of the loaded assembly is on its frame when the context unloads.
        [MethodImpl(MethodImplOptions.NoInlining)]
        private static void CheckSettersIn(MapLoadContext context, string assemblyPath)
        {
            SetterScan.Check(context.Open(assemblyPath));
        }

        /// <summary>Loads the built assembly, constructs every declared object class and collects what each changed from its standard object.</summary>
        /// <param name="assemblyPath">The map's built assembly; <c>Moonwell.Objects.dll</c> is beside it.</param>
        /// <param name="project">The map project; lines in messages are relative to its folder.</param>
        /// <param name="standard">The standard values.</param>
        /// <param name="set">The tables that fields a class leaves unset read (<see cref="GameData.ValueSet"/>): null for the base tables, else <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>.</param>
        /// <returns>The declared objects, ordered by class name, the ids of the standard classes their lists and references name, the classes the map's Lua uses, the hero bonuses and the fields read of a type's default object; all empty when the map does not use <c>Moonwell.Objects</c>.</returns>
        /// <exception cref="BuildException">A constructor calls the game or throws, a declared field or a unit's level is set outside a constructor, a hero's total or level is not one it can start with, or <c>[Replaces]</c> is misused.</exception>
        public static DeclarationSet Read(string assemblyPath, MapProject project, StandardData standard, string? set)
        {
            if (!File.Exists(assemblyPath))
            {
                throw new BuildException($"the built assembly does not exist: {assemblyPath}");
            }

            var context = new MapLoadContext(Path.GetDirectoryName(Path.GetFullPath(assemblyPath))!);
            try
            {
                return ReadIn(context, assemblyPath, project, standard, set);
            }
            finally
            {
                context.Unload();
            }
        }

        // Kept apart from Read so that nothing of the loaded assembly is on Read's frame when the context unloads.
        [MethodImpl(MethodImplOptions.NoInlining)]
        private static DeclarationSet ReadIn(MapLoadContext context, string assemblyPath, MapProject project, StandardData standard, string? set)
        {
            var map = context.Open(assemblyPath);
            SetterScan.Check(map);
            if (!map.GetReferencedAssemblies().Any(name => name.Name == ObjectsAssemblyName))
            {
                return new DeclarationSet(Array.Empty<DeclaredObject>(), new Dictionary<string, string>(), Array.Empty<UsedClass>(), string.Empty, new HeroBonus(0f, 0f), new Dictionary<string, IReadOnlyList<string>>());
            }

            var objects = context.LoadFromAssemblyName(new AssemblyName(ObjectsAssemblyName));
            var used = ObjectLua.UsedClasses(project, map, objects, assemblyPath, out var support);

            var classNames = new Dictionary<string, string>(StringComparer.Ordinal);
            var standardClasses = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var type in objects.GetTypes().OrderBy(type => type.FullName, StringComparer.Ordinal))
            {
                var id = StandardIdOf(type);
                if (id != null)
                {
                    classNames.TryAdd(id, MapClasses.Describe(type));
                    standardClasses[MapClasses.Describe(type)] = id;
                }
            }

            var candidates = map.GetTypes()
                .Where(IsDeclaredClass)
                .OrderBy(type => MapClasses.Describe(type), StringComparer.Ordinal)
                .ToList();
            CheckReplaces(candidates);

            // A hero's MaxLife, MaxMana and Mana read in a constructor are its totals, by the map's gameplay constants. The data of a map
            // with no objects has none, and reads them as its base fields.
            var bonus = ReferenceEquals(standard, StandardData.Empty) ? new HeroBonus(0f, 0f) : MapMisc.Read(project.MapFolder, standard.Misc(set));
            var standardValues = objects.GetType("Moonwell.Objects.StandardValues", true)!;
            var values = standardValues.GetProperty("Read", BindingFlags.Public | BindingFlags.Static)!;
            var heroBonus = standardValues.GetProperty("HeroBonus", BindingFlags.Public | BindingFlags.Static)!;
            values.SetValue(null, (Func<string, string, int, string?>)((id, field, level) => standard.Value(id, field, level, set)));
            heroBonus.SetValue(null, (Func<(float LifePerStrength, float ManaPerIntelligence)>)(() => (bonus.LifePerStrength, bonus.ManaPerIntelligence)));
            try
            {
                var comparer = new ChangedFields(standard, set, project, classNames, bonus);
                var declared = new List<DeclaredObject>();
                foreach (var type in candidates)
                {
                    declared.Add(Construct(type, comparer, project));
                }

                var named = new SortedDictionary<string, string>(StringComparer.Ordinal);
                foreach (var one in declared)
                {
                    foreach (var reference in one.References)
                    {
                        if (standardClasses.TryGetValue(reference, out var standardId))
                        {
                            named[reference] = standardId;
                        }
                    }
                }

                return new DeclarationSet(declared, named, used, support, bonus, ObjectLua.TypeFields(objects, assemblyPath));
            }
            finally
            {
                values.SetValue(null, null);
                heroBonus.SetValue(null, null);
            }
        }

        private static DeclaredObject Construct(Type type, ChangedFields comparer, MapProject project)
        {
            var name = MapClasses.Describe(type);
            var kindBase = KindBaseOf(type)!;
            var kind = KindOfBase[kindBase.FullName!];
            var root = RootOf(type);
            if (root == null)
            {
                throw new BuildException($"{name} inherits {MapClasses.Describe(kindBase)} without a standard object; inherit one of the standard classes");
            }

            var constructor = type.GetConstructor(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic, null, Type.EmptyTypes, null);
            if (constructor == null)
            {
                throw new BuildException($"{name} needs a constructor without parameters");
            }

            object instance;
            try
            {
                instance = constructor.Invoke(null);
            }
            catch (TargetInvocationException wrapped) when (wrapped.InnerException != null)
            {
                throw MapClasses.ConstructionError(wrapped.InnerException, name, project);
            }

            var put = (IReadOnlyDictionary<string, object?>)kindBase.GetProperty("Declared", BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(instance)!;
            if (kind == "hero")
            {
                put = comparer.WithBases(type, root, put, Internal<IReadOnlyDictionary<string, object?>>(kindBase, instance, "Totals"), TotalsPutAt(kindBase, instance));
            }

            var references = new SortedSet<string>(StringComparer.Ordinal);
            var changed = comparer.Compute(name, kind, root, put, references);

            // The game starts a hero at level 1; its ulev is the standard hero's (World Editor's Level), which only its Set reaches.
            if (kind == "hero" && changed.ContainsKey("ulev"))
            {
                throw new BuildException($"{name} changes ulev: a hero starts at level 1; its level is not declared");
            }

            return new DeclaredObject(
                name,
                kind,
                root,
                HasAttribute(type, ReplacesAttributeName),
                PinnedIdOf(type),
                CarrierIdOf(type),
                changed,
                references.ToList());
        }

        // An internal property of a kind base, read as the build reads Declared.
        private static T Internal<T>(Type kindBase, object instance, string property)
        {
            return (T)kindBase.GetProperty(property, BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(instance)!;
        }

        // Where each of a hero's totals was put: Declared's TotalsPutAt, reached through the kind base's Fields.
        private static IReadOnlyDictionary<string, object?> TotalsPutAt(Type kindBase, object instance)
        {
            var fields = Internal<object>(kindBase, instance, "Fields");
            return (IReadOnlyDictionary<string, object?>)fields.GetType().GetProperty("TotalsPutAt", BindingFlags.Instance | BindingFlags.NonPublic)!.GetValue(fields)!;
        }

        // A class that is constructed: not abstract or open generic, not a standard class, and its base chain reaches a kind base.
        internal static bool IsDeclaredClass(Type type)
        {
            return !type.IsAbstract && !type.IsGenericTypeDefinition && StandardIdOf(type) == null && KindBaseOf(type) != null;
        }

        internal static Type? KindBaseOf(Type type)
        {
            for (var parent = type.BaseType; parent != null; parent = parent.BaseType)
            {
                if (parent.FullName != null && KindOfBase.ContainsKey(parent.FullName))
                {
                    return parent;
                }
            }

            return null;
        }

        private static void CheckReplaces(List<Type> candidates)
        {
            var replaced = new Dictionary<Type, Type>();
            foreach (var type in candidates)
            {
                if (!HasAttribute(type, ReplacesAttributeName))
                {
                    continue;
                }

                var parent = type.BaseType!;
                if (StandardIdOf(parent) == null)
                {
                    throw new BuildException($"{MapClasses.Describe(type)} replaces {MapClasses.Describe(parent)}, which is not a standard object");
                }

                if (replaced.TryGetValue(parent, out var first))
                {
                    throw new BuildException($"{MapClasses.Describe(first)} and {MapClasses.Describe(type)} both replace {MapClasses.Describe(parent)}");
                }

                replaced[parent] = type;
            }
        }

        // The standard id of the nearest class in the base chain that is a standard object.
        private static string? RootOf(Type type)
        {
            for (var parent = type.BaseType; parent != null; parent = parent.BaseType)
            {
                var id = StandardIdOf(parent);
                if (id != null)
                {
                    return id;
                }
            }

            return null;
        }

        internal static string? StandardIdOf(Type type)
        {
            return FirstArgument(type, StandardAttributeName);
        }

        private static string? PinnedIdOf(Type type)
        {
            return FirstArgument(type, ObjectIdAttributeName);
        }

        private static string? CarrierIdOf(Type type)
        {
            foreach (var data in type.GetCustomAttributesData())
            {
                if (data.AttributeType.FullName == ObjectIdAttributeName)
                {
                    foreach (var named in data.NamedArguments)
                    {
                        if (named.MemberName == "Carrier")
                        {
                            return (string?)named.TypedValue.Value;
                        }
                    }
                }
            }

            return null;
        }

        // Attributes are read by name, since the map's copy of Moonwell.Objects is not the build's own.
        private static string? FirstArgument(Type type, string attribute)
        {
            foreach (var data in type.GetCustomAttributesData())
            {
                if (data.AttributeType.FullName == attribute)
                {
                    return data.ConstructorArguments.Count == 0 ? null : (string?)data.ConstructorArguments[0].Value;
                }
            }

            return null;
        }

        private static bool HasAttribute(Type type, string attribute)
        {
            return type.GetCustomAttributesData().Any(data => data.AttributeType.FullName == attribute);
        }
    }
}
