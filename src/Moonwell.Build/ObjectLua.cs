using System.Globalization;
using System.Reflection;
using System.Reflection.Metadata;
using System.Reflection.PortableExecutable;
using System.Text;
using System.Text.RegularExpressions;

namespace Moonwell.Build
{
    /// <summary>
    /// The Lua of the classes of <c>Moonwell.Objects</c> the map uses. CSharp.lua leaves those classes out (each is marked
    /// <c>@CSharpLua.Ignore</c>), so the build writes them as a library, adds them to <c>System.init</c>, and appends the values
    /// their fields read in the game.
    /// </summary>
    public static class ObjectLua
    {
        private const string FieldAttributeName = "Moonwell.Objects.FieldAttribute";
        // The library's last line: a statement, not a comment, so that a release build keeps it. The compiled code is after it.
        private const string LibraryEnd = "local endOfMoonwellObjects = true";
        private const string ObjectsNamespace = "Moonwell.Objects";

        // The generic types whose names the ignored files' preambles evaluate when the script loads.
        private static readonly string[] Generics = { "Levels_1", "ObjectList_1", "ObjectRef_1" };

        private static readonly Regex Getter = new Regex("\\bget[A-Za-z_][A-Za-z0-9_]*", RegexOptions.CultureInvariant);

        /// <summary>
        /// The declared classes of the map, every class of <c>Moonwell.Objects</c> the map assembly's type references name that is
        /// a kind base or inherits one, and their bases up to the kind base; each base before the classes that inherit it. A class
        /// of <c>Moonwell.Objects</c> keeps the fields and methods the assembly's member references name.
        /// </summary>
        /// <param name="map">The map assembly, loaded in the build's load context.</param>
        /// <param name="objects">The map's copy of <c>Moonwell.Objects</c>, in the same context.</param>
        /// <param name="assemblyPath">The map assembly's file, whose metadata holds the type references.</param>
        /// <returns>The used classes.</returns>
        internal static IReadOnlyList<UsedClass> UsedClasses(Assembly map, Assembly objects, string assemblyPath)
        {
            var starts = new List<Type>();
            foreach (var type in map.GetTypes())
            {
                if (Declarations.IsDeclaredClass(type))
                {
                    starts.Add(type);
                }
            }

            // The names of the members each class of Moonwell.Objects is referenced for, a property by its own name.
            var members = new Dictionary<string, HashSet<string>>(StringComparer.Ordinal);
            using (var pe = new PEReader(new MemoryStream(File.ReadAllBytes(assemblyPath))))
            {
                var reader = pe.GetMetadataReader();
                foreach (var handle in reader.TypeReferences)
                {
                    var name = ObjectsTypeName(reader, handle);
                    var type = name == null ? null : objects.GetType(name);
                    if (type != null && (IsKindBase(type) || Declarations.KindBaseOf(type) != null))
                    {
                        starts.Add(type);
                    }
                }

                foreach (var handle in reader.MemberReferences)
                {
                    var member = reader.GetMemberReference(handle);
                    var parent = member.Parent.Kind == HandleKind.TypeReference ? ObjectsTypeName(reader, (TypeReferenceHandle)member.Parent) : null;
                    if (parent == null)
                    {
                        continue;
                    }

                    var name = reader.GetString(member.Name);
                    if (name.StartsWith("get_", StringComparison.Ordinal) || name.StartsWith("set_", StringComparison.Ordinal))
                    {
                        name = name.Substring(4);
                    }

                    if (!members.TryGetValue(parent, out var names))
                    {
                        members[parent] = names = new HashSet<string>(StringComparer.Ordinal);
                    }

                    names.Add(name);
                }
            }

            var used = new List<UsedClass>();
            var seen = new HashSet<string>(StringComparer.Ordinal);
            foreach (var type in starts.OrderBy(MapClasses.Describe, StringComparer.Ordinal))
            {
                Add(type, used, seen, members);
            }

            return used;
        }

        /// <summary>
        /// The library of the used classes of <c>Moonwell.Objects</c>: the namespace tables and the generic types the compiled
        /// files evaluate when they load, then each class with its base, its methods the map calls (such as <c>Set</c>, which does
        /// nothing, and <c>Levels</c>, which gives nil) and, for each field of it the map names, a getter that reads the class's
        /// values (<c>MoonwellField</c> in <c>moonwell.lua</c>) and a setter that does nothing: the fields' values are written into
        /// the object files, so what a constructor puts has no effect in the game. The members are those of the class's stub
        /// (<see cref="ObjectStubs"/>), the only ones CSharp.lua can call.
        /// </summary>
        /// <param name="used">The used classes.</param>
        /// <returns>The library's Lua.</returns>
        public static string Definitions(IReadOnlyList<UsedClass> used)
        {
            var text = new StringBuilder();
            text.Append("-- The classes of Moonwell.Objects the map uses, written by the build: CSharp.lua leaves them out.\n");
            var spaces = new SortedSet<string>(StringComparer.Ordinal) { ObjectsNamespace };
            foreach (var one in used)
            {
                if (one.InObjects)
                {
                    for (var space = NamespaceOf(one.FullName); space.Length > ObjectsNamespace.Length; space = NamespaceOf(space))
                    {
                        spaces.Add(space);
                    }
                }
            }

            text.Append("Moonwell = Moonwell or {}\n");
            foreach (var space in spaces)
            {
                text.Append(space).Append(" = ").Append(space).Append(" or {}\n");
            }

            text.Append("local function generic() return {} end\n");
            foreach (var generic in Generics)
            {
                text.Append(ObjectsNamespace).Append('.').Append(generic).Append(" = generic\n");
            }

            text.Append("local function nothing() end\n");
            text.Append("local function none() return nil end\n");
            foreach (var group in used.Where(one => one.InObjects).GroupBy(one => NamespaceOf(one.FullName)).OrderBy(group => group.Key, StringComparer.Ordinal))
            {
                text.Append("System.namespace(\"").Append(group.Key).Append("\", function (namespace)\n");
                foreach (var one in group)
                {
                    WriteClass(text, one);
                }

                text.Append("end)\n");
            }

            text.Append(LibraryEnd).Append('\n');
            return text.ToString();
        }

        /// <summary>Adds the used classes of <c>Moonwell.Objects</c> to the types of <c>System.init</c>: each right after the class of the runtime its kind base inherits, bases first.</summary>
        /// <param name="script">The compiled script.</param>
        /// <param name="used">The used classes.</param>
        /// <returns>The script with the classes in its types.</returns>
        /// <exception cref="BuildException">The script has no types list, or it lacks the class of the runtime a kind base inherits.</exception>
        public static string AddToInit(string script, IReadOnlyList<UsedClass> used)
        {
            if (!used.Any(one => one.InObjects))
            {
                return script;
            }

            var init = script.IndexOf("System.init({", StringComparison.Ordinal);
            var open = init < 0 ? -1 : script.IndexOf("types = {", init, StringComparison.Ordinal);
            if (open < 0)
            {
                throw new BuildException("the compiled script has no System.init types list");
            }

            open += "types = {".Length;
            var close = script.IndexOf('}', open);
            var block = script.Substring(open, close - open);
            var names = Regex.Matches(block, "\"([^\"]+)\"").Select(match => match.Groups[1].Value).ToList();
            var first = Regex.Match(block, "(\\r?\\n)([ \\t]*)\"");
            var newline = first.Success ? first.Groups[1].Value : "\n";
            var indent = first.Success ? first.Groups[2].Value : "      ";
            var closing = Regex.Match(block, "\\r?\\n([ \\t]*)$");

            var byName = used.ToDictionary(one => one.FullName, StringComparer.Ordinal);
            var after = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var one in used)
            {
                if (!one.InObjects)
                {
                    continue;
                }

                var anchor = RuntimeBaseOf(one, byName);
                var at = after.TryGetValue(anchor, out var last) ? names.IndexOf(last) : names.IndexOf(anchor);
                if (at < 0)
                {
                    throw new BuildException($"the compiled script's System.init does not list {anchor}, which {one.FullName} inherits");
                }

                names.Insert(at + 1, one.FullName);
                after[anchor] = one.FullName;
            }

            var list = new StringBuilder();
            for (var i = 0; i < names.Count; i++)
            {
                list.Append(newline).Append(indent).Append('"').Append(names[i]).Append('"');
                if (i < names.Count - 1)
                {
                    list.Append(',');
                }
            }

            list.Append(newline).Append(closing.Success ? closing.Groups[1].Value : string.Empty);
            return script.Substring(0, open) + list + script.Substring(close);
        }

        /// <summary>
        /// Appends <c>MoonwellObjectValues</c> to the script: for each used class that has an object (a declared class or a standard
        /// class), the values of the fields its chain declares whose getter the compiled code calls. A declared class has its
        /// changed values, else its standard root's; a standard class its own. Lists and references hold the objects' ids; a field
        /// with no value has its type's default, a string the empty text.
        /// </summary>
        /// <param name="script">The compiled script, with the library before its compiled code; the fields read are found after the library.</param>
        /// <param name="used">The used classes.</param>
        /// <param name="declared">The declared objects.</param>
        /// <param name="ids">The object id of each declared class, and of each standard class the declarations name.</param>
        /// <param name="standard">The standard data.</param>
        /// <param name="set">The tables standard values are read from (<see cref="GameData.ValueSet"/>): null for the base tables.</param>
        /// <returns>The script with the values table at its end.</returns>
        /// <exception cref="BuildException">A declared list or reference names a class that has no object id.</exception>
        public static string Values(
            string script,
            IReadOnlyList<UsedClass> used,
            IReadOnlyList<DeclaredObject> declared,
            IReadOnlyDictionary<string, string> ids,
            StandardData standard,
            string? set)
        {
            var read = GettersCalled(script);
            var byName = used.ToDictionary(one => one.FullName, StringComparer.Ordinal);
            var objects = declared.ToDictionary(one => one.ClassName, StringComparer.Ordinal);
            var none = new Dictionary<string, object?>(StringComparer.Ordinal);
            var text = new StringBuilder(script);
            if (script.Length > 0 && script[script.Length - 1] != '\n')
            {
                text.Append('\n');
            }

            text.Append("MoonwellObjectValues = {\n");
            foreach (var one in used.OrderBy(one => one.FullName, StringComparer.Ordinal))
            {
                string root;
                IReadOnlyDictionary<string, object?> changed;
                if (objects.TryGetValue(one.FullName, out var declaredObject))
                {
                    root = declaredObject.StandardRoot;
                    changed = declaredObject.Changed;
                }
                else if (one.StandardId != null)
                {
                    root = one.StandardId;
                    changed = none;
                }
                else
                {
                    continue;
                }

                var reader = new FieldValues(one, root, changed, ids, standard, set);
                var entries = new SortedDictionary<string, string>(StringComparer.Ordinal);
                for (var chain = one; chain != null; chain = byName.GetValueOrDefault(chain.BaseFullName))
                {
                    foreach (var field in chain.Fields)
                    {
                        if (read.Contains("get" + field.Property) && !entries.ContainsKey(field.FieldId))
                        {
                            entries[field.FieldId] = reader.Literal(field);
                        }
                    }
                }

                text.Append("  [").Append(Quote(one.FullName)).Append("] = {");
                var separator = " ";
                foreach (var entry in entries)
                {
                    text.Append(separator).Append('[').Append(Quote(entry.Key)).Append("] = ").Append(entry.Value);
                    separator = ", ";
                }

                text.Append(entries.Count > 0 ? " },\n" : "},\n");
            }

            text.Append("}\n");
            return text.ToString();
        }

        /// <summary>A Lua string literal.</summary>
        internal static string Quote(string value)
        {
            var text = new StringBuilder("\"");
            foreach (var character in value)
            {
                switch (character)
                {
                    case '\\':
                        text.Append("\\\\");
                        break;
                    case '"':
                        text.Append("\\\"");
                        break;
                    case '\n':
                        text.Append("\\n");
                        break;
                    case '\r':
                        text.Append("\\r");
                        break;
                    default:
                        if (character < ' ' || character == '\x7f')
                        {
                            text.Append('\\').Append(((int)character).ToString("D3", CultureInfo.InvariantCulture));
                        }
                        else
                        {
                            text.Append(character);
                        }

                        break;
                }
            }

            return text.Append('"').ToString();
        }

        /// <summary>The object id as a number, the first character the highest byte, as <c>ObjectTypes.FourCC</c> makes it.</summary>
        internal static int FourCC(string id)
        {
            return ((id[0] * 256 + id[1]) * 256 + id[2]) * 256 + id[3];
        }

        private static void Add(Type type, List<UsedClass> used, HashSet<string> seen, IReadOnlyDictionary<string, HashSet<string>> members)
        {
            var name = MapClasses.Describe(type);
            if (seen.Contains(name))
            {
                return;
            }

            seen.Add(name);
            var kindBase = IsKindBase(type) ? type : Declarations.KindBaseOf(type)!;
            if (type != kindBase)
            {
                Add(type.BaseType!, used, seen, members);
            }

            var inObjects = type.Assembly.GetName().Name == Declarations.ObjectsAssemblyName;
            IReadOnlySet<string> names = members.TryGetValue(name, out var referenced) ? referenced : new HashSet<string>();
            used.Add(new UsedClass(
                name,
                MapClasses.Describe(type.BaseType!),
                Declarations.KindOfBase[kindBase.FullName!],
                Declarations.StandardIdOf(type),
                inObjects,
                inObjects ? FieldsOf(type, names) : Array.Empty<UsedField>(),
                inObjects ? ObjectStubs.Methods(type, names) : Array.Empty<UsedMethod>(),
                inObjects ? ObjectStubs.Class(type, names) : string.Empty));
        }

        // The full name of a type a reference names in Moonwell.Objects; null for a type of another assembly or a nested type.
        private static string? ObjectsTypeName(MetadataReader reader, TypeReferenceHandle handle)
        {
            var reference = reader.GetTypeReference(handle);
            if (reference.ResolutionScope.Kind != HandleKind.AssemblyReference)
            {
                return null;
            }

            var scope = reader.GetAssemblyReference((AssemblyReferenceHandle)reference.ResolutionScope);
            if (reader.GetString(scope.Name) != Declarations.ObjectsAssemblyName)
            {
                return null;
            }

            var space = reader.GetString(reference.Namespace);
            var name = reader.GetString(reference.Name);
            return space.Length == 0 ? name : space + "." + name;
        }

        private static List<UsedField> FieldsOf(Type type, IReadOnlySet<string> names)
        {
            var fields = new List<UsedField>();
            var properties = type.GetProperties(BindingFlags.Instance | BindingFlags.Public | BindingFlags.DeclaredOnly)
                .Where(property => names.Contains(property.Name))
                .OrderBy(property => property.MetadataToken);
            foreach (var property in properties)
            {
                var attribute = property.GetCustomAttributesData().FirstOrDefault(data => data.AttributeType.FullName == FieldAttributeName);
                if (attribute == null)
                {
                    continue;
                }

                var generic = property.PropertyType.IsGenericType ? property.PropertyType.GetGenericTypeDefinition().FullName : null;
                var access = generic switch
                {
                    "Moonwell.Objects.Levels`1" => "levels",
                    "Moonwell.Objects.ObjectList`1" => "list",
                    "Moonwell.Objects.ObjectRef`1" => "ref",
                    _ => "value",
                };
                fields.Add(new UsedField(property.Name, (string)attribute.ConstructorArguments[0].Value!, access));
            }

            return fields;
        }

        private static bool IsKindBase(Type type)
        {
            return type.FullName != null && Declarations.KindOfBase.ContainsKey(type.FullName);
        }

        private static void WriteClass(StringBuilder text, UsedClass one)
        {
            text.Append("  namespace.class(\"").Append(one.FullName.Substring(NamespaceOf(one.FullName).Length + 1)).Append("\", function (namespace)\n");
            text.Append("    return {\n");
            text.Append("      base = function (out)\n");
            text.Append("        return { out.").Append(one.BaseFullName).Append(" }\n");
            text.Append("      end");
            foreach (var method in one.Methods)
            {
                text.Append(",\n      ").Append(method.LuaName).Append(method.Void ? " = nothing" : " = none");
            }

            foreach (var field in one.Fields)
            {
                text.Append(",\n      get").Append(field.Property).Append(" = function (this) return MoonwellField(this, ")
                    .Append(Quote(field.FieldId)).Append(", ").Append(Quote(field.Access)).Append(") end");
                if (field.Access is "value" or "levels")
                {
                    text.Append(",\n      set").Append(field.Property).Append(" = nothing");
                }
            }

            text.Append("\n    }\n");
            text.Append("  end)\n");
        }

        // The class of the runtime at the top of a used class's chain in Moonwell.Objects, such as Moonwell.Hero.
        private static string RuntimeBaseOf(UsedClass one, IReadOnlyDictionary<string, UsedClass> byName)
        {
            var chain = one;
            while (byName.TryGetValue(chain.BaseFullName, out var parent) && parent.InObjects)
            {
                chain = parent;
            }

            return chain.BaseFullName;
        }

        private static string NamespaceOf(string fullName)
        {
            var dot = fullName.LastIndexOf('.');
            return dot < 0 ? string.Empty : fullName.Substring(0, dot);
        }

        // The getter names the compiled code holds: the script after the objects' library, whose own getters are left out. A
        // release build writes no header before the compiled code, so the library's last line marks where it starts.
        private static HashSet<string> GettersCalled(string script)
        {
            var end = script.IndexOf(LibraryEnd, StringComparison.Ordinal);
            var code = end < 0 ? script : script.Substring(end + LibraryEnd.Length);
            var names = new HashSet<string>(StringComparer.Ordinal);
            foreach (Match match in Getter.Matches(code))
            {
                names.Add(match.Value);
            }

            return names;
        }
    }
}
