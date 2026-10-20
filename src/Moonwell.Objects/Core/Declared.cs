using System;
using System.Collections.Generic;

namespace Moonwell.Objects
{
    /// <summary>The values a declared object puts on its fields while its constructors run, on .NET.</summary>
    /// <remarks>
    /// <para>
    /// A standard class's constructor sets the values its object has in the base tables (World Editor's Default). Those
    /// are the standard object's, not declared: they are kept apart from <see cref="Values"/>, never checked, and a value
    /// the map's class puts wins over them. A field the map's class leaves unset reads the build's values
    /// (<see cref="StandardValues.Read"/>, the map's data set) and, when no build gives them, the constructor's.
    /// </para>
    /// <para>
    /// What the standard class's constructors put is told apart by count: the standard class is made once by itself, and
    /// the puts it makes (<see cref="Put{T}"/>, <see cref="PutLevel"/>, <see cref="GetList{T}"/> and <see cref="GetRef{T}"/>
    /// each count one) are the first puts of every object of a class that inherits it. Its constructors run first and do
    /// the same each time; a map's class sets its fields in its own constructor, after them.
    /// </para>
    /// </remarks>
    /// @CSharpLua.Ignore
    public sealed class Declared
    {
        private static readonly Lazy<Dictionary<string, Type>> StandardClasses = new Lazy<Dictionary<string, Type>>(FindStandardClasses);

        // The number of puts each standard class's constructors make, by the class's assembly-qualified name. Its values are
        // typed object, and its key a string: CSharp.lua writes Lua for the generic types a file uses even when its class is
        // ignored, and this file's other dictionaries are of these types already.
        private static readonly Dictionary<string, object?> standardPuts = new Dictionary<string, object?>();

        // The standard class being made by itself on this thread to count its puts, and the count so far.
        [ThreadStatic]
        private static Type? counting;

        [ThreadStatic]
        private static int counted;

        private readonly Dictionary<string, object?> values = new Dictionary<string, object?>();
        private readonly Dictionary<string, object?> standard = new Dictionary<string, object?>();
        private readonly object? owner;

        // The puts still to come from the standard class's constructors: 0 when they are done, -1 while the standard class
        // is being made by itself to count them.
        private int standardLeft;

        /// <summary>Holds the values of an object whose puts are not checked.</summary>
        public Declared()
        {
        }

        /// <summary>Holds the values of <paramref name="owner"/>, checking each put against the rules of its field's property (see <see cref="FieldRuleException"/>).</summary>
        public Declared(object owner)
        {
            this.owner = owner;
            var type = owner.GetType();
            var standardClass = StandardClassOf(type);
            if (standardClass == null)
            {
                standardLeft = 0;
            }
            else if (type == counting)
            {
                standardLeft = -1;
            }
            else
            {
                standardLeft = StandardPutsOf(standardClass);
            }
        }

        /// <summary>The fields that were put (and the lists and references that were asked for), by field id; what a standard class's constructor puts is not among them.</summary>
        public IReadOnlyDictionary<string, object?> Values
        {
            get { return values; }
        }

        /// <summary>Gives the value put on a field, else the standard object's value of the field.</summary>
        public T Get<T>(object owner, string field)
        {
            if (values.TryGetValue(field, out var put))
            {
                return (T)put!;
            }

            return ReadStandard<T>(owner, field, 0);
        }

        /// <summary>Puts a value on a field; a value its rules do not allow throws a <see cref="FieldRuleException"/>, and a formula of levels is checked when its levels are counted.</summary>
        public void Put<T>(string field, T value)
        {
            if (FromStandard())
            {
                standard[field] = value;
                return;
            }

            if (owner != null)
            {
                RuleChecks.Put(owner, field, null, value);
            }

            values[field] = value;
        }

        /// <summary>Puts the value of one level of a field, kept under <c>field:level</c>; a value its rules do not allow throws a <see cref="FieldRuleException"/> naming the level.</summary>
        public void PutLevel(string field, int level, object value)
        {
            if (FromStandard())
            {
                standard[Key(field, level)] = value;
                return;
            }

            if (owner != null)
            {
                RuleChecks.Put(owner, field, level, value);
            }

            values[Key(field, level)] = value;
        }

        /// <summary>Gives the per-level values put on a field, else the standard object's values, read level by level.</summary>
        public Levels<T> GetLevels<T>(object owner, string field)
        {
            if (values.TryGetValue(field, out var put))
            {
                return (Levels<T>)put!;
            }

            if (StandardValues.Read == null && standard.TryGetValue(field, out var constructed))
            {
                return (Levels<T>)constructed!;
            }

            return new Levels<T>(level => ReadStandard<T>(owner, field, level));
        }

        /// <summary>Gives the list of a field, made the first time it is asked for as the standard object's list; a field with levels starts as level 1's.</summary>
        public ObjectList<T> GetList<T>(object owner, string field)
        {
            if (FromStandard())
            {
                if (!standard.TryGetValue(field, out var building))
                {
                    building = new ObjectList<T>();
                    standard[field] = building;
                }

                return (ObjectList<T>)building!;
            }

            if (!values.TryGetValue(field, out var list))
            {
                IReadOnlyList<object> entries;
                if (StandardValues.Read != null)
                {
                    entries = StandardEntries(ReadText(owner, field, 0) ?? ReadText(owner, field, 1));
                }
                else
                {
                    entries = standard.TryGetValue(field, out var constructed) ? ((ObjectList<T>)constructed!).Entries : new List<object>();
                }

                var made = new ObjectList<T>(entries);
                if (this.owner != null)
                {
                    var checkedOwner = this.owner;
                    made.Grew = count => RuleChecks.Entries(checkedOwner, field, count);
                }

                list = made;
                values[field] = list;
            }

            return (ObjectList<T>)list!;
        }

        /// <summary>Gives the reference of a field, made the first time it is asked for: empty, or the standard class's constructor's when no build gives the values.</summary>
        public ObjectRef<T> GetRef<T>(string field)
        {
            if (FromStandard())
            {
                if (!standard.TryGetValue(field, out var building))
                {
                    building = new ObjectRef<T>();
                    standard[field] = building;
                }

                return (ObjectRef<T>)building!;
            }

            if (!values.TryGetValue(field, out var reference))
            {
                var made = new ObjectRef<T>();
                if (StandardValues.Read == null && standard.TryGetValue(field, out var constructed))
                {
                    made.Entry = ((ObjectRef<T>)constructed!).Entry;
                }

                reference = made;
                values[field] = reference;
            }

            return (ObjectRef<T>)reference!;
        }

        // Counts one put, and tells whether it is the standard class's constructors'.
        private bool FromStandard()
        {
            if (standardLeft == 0)
            {
                return false;
            }

            if (standardLeft > 0)
            {
                standardLeft--;
            }
            else
            {
                counted++;
            }

            return true;
        }

        private static string Key(string field, int level)
        {
            return field + ":" + level.ToString(System.Globalization.CultureInfo.InvariantCulture);
        }

        private static List<object> StandardEntries(string? text)
        {
            var entries = new List<object>();
            if (!string.IsNullOrEmpty(text))
            {
                foreach (var id in text.Split(','))
                {
                    entries.Add(StandardClasses.Value.TryGetValue(id, out var type) ? type : id);
                }
            }

            return entries;
        }

        // The standard object's value of a field, or of one level of it (0 for none): the build's, else the standard class's
        // constructor's when no build gives the values, else the type's default.
        private T ReadStandard<T>(object owner, string field, int level)
        {
            if (StandardValues.Read != null)
            {
                return StandardValues.Parse<T>(ReadText(owner, field, level));
            }

            return standard.TryGetValue(level == 0 ? field : Key(field, level), out var constructed) ? (T)constructed! : StandardValues.Parse<T>(null);
        }

        private static string? ReadText(object owner, string field, int level)
        {
            var read = StandardValues.Read;
            var id = StandardIdOf(owner.GetType());
            return read == null || id == null ? null : read(id, field, level);
        }

        // The number of puts a standard class's constructors make, found by making the class by itself the first time.
        private static int StandardPutsOf(Type standardClass)
        {
            var name = standardClass.AssemblyQualifiedName ?? standardClass.FullName ?? standardClass.Name;
            lock (standardPuts)
            {
                if (standardPuts.TryGetValue(name, out var known))
                {
                    return (int)known!;
                }
            }

            var outerClass = counting;
            var outerCount = counted;
            counting = standardClass;
            counted = 0;
            int count;
            try
            {
                Activator.CreateInstance(standardClass, true);
                count = counted;
            }
            catch (MissingMethodException)
            {
                // A standard class with no constructor without parameters puts nothing a map's class could inherit.
                count = 0;
            }
            catch (MemberAccessException)
            {
                // An abstract standard class is made by no map's class directly; its constructors put nothing here.
                count = 0;
            }
            finally
            {
                counting = outerClass;
                counted = outerCount;
            }

            lock (standardPuts)
            {
                standardPuts[name] = count;
            }

            return count;
        }

        /// <summary>The standard classes of this assembly by the id they stand for.</summary>
        private static Dictionary<string, Type> FindStandardClasses()
        {
            var classes = new Dictionary<string, Type>();
            foreach (var type in typeof(Declared).Assembly.GetTypes())
            {
                var standard = (StandardAttribute?)Attribute.GetCustomAttribute(type, typeof(StandardAttribute), false);
                if (standard != null)
                {
                    classes[standard.Id] = type;
                }
            }

            return classes;
        }

        /// <summary>The nearest class in the base chain that is a standard object; null when none is.</summary>
        private static Type? StandardClassOf(Type? type)
        {
            while (type != null)
            {
                if (Attribute.GetCustomAttribute(type, typeof(StandardAttribute), false) != null)
                {
                    return type;
                }

                type = type.BaseType;
            }

            return null;
        }

        /// <summary>The id of the nearest class in the base chain that is a standard object; null when none is.</summary>
        private static string? StandardIdOf(Type? type)
        {
            var standardClass = StandardClassOf(type);
            return standardClass == null ? null : ((StandardAttribute)Attribute.GetCustomAttribute(standardClass, typeof(StandardAttribute), false)!).Id;
        }
    }
}
