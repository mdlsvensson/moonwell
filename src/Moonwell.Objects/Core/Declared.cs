using System;
using System.Collections.Generic;

namespace Moonwell.Objects
{
    /// <summary>The values a declared object puts on its fields while its constructors run, on .NET.</summary>
    /// @CSharpLua.Ignore
    public sealed class Declared
    {
        private static readonly Lazy<Dictionary<string, Type>> StandardClasses = new Lazy<Dictionary<string, Type>>(FindStandardClasses);

        private readonly Dictionary<string, object?> values = new Dictionary<string, object?>();
        private readonly object? owner;

        /// <summary>Holds the values of an object whose puts are not checked.</summary>
        public Declared()
        {
        }

        /// <summary>Holds the values of <paramref name="owner"/>, checking each put against the rules of its field's property (see <see cref="FieldRuleException"/>).</summary>
        public Declared(object owner)
        {
            this.owner = owner;
        }

        /// <summary>The fields that were put (and the lists and references that were asked for), by field id.</summary>
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

            return StandardValues.Parse<T>(ReadStandard(owner, field, 0));
        }

        /// <summary>Puts a value on a field; a value its rules do not allow throws a <see cref="FieldRuleException"/>, and a formula of levels is checked when its levels are counted.</summary>
        public void Put<T>(string field, T value)
        {
            if (owner != null)
            {
                RuleChecks.Put(owner, field, null, value);
            }

            values[field] = value;
        }

        /// <summary>Puts the value of one level of a field, kept under <c>field:level</c>; a value its rules do not allow throws a <see cref="FieldRuleException"/> naming the level.</summary>
        public void PutLevel(string field, int level, object value)
        {
            if (owner != null)
            {
                RuleChecks.Put(owner, field, level, value);
            }

            values[field + ":" + level.ToString(System.Globalization.CultureInfo.InvariantCulture)] = value;
        }

        /// <summary>Gives the per-level values put on a field, else the standard object's values, read level by level.</summary>
        public Levels<T> GetLevels<T>(object owner, string field)
        {
            if (values.TryGetValue(field, out var put))
            {
                return (Levels<T>)put!;
            }

            return new Levels<T>(level => StandardValues.Parse<T>(ReadStandard(owner, field, level)));
        }

        /// <summary>Gives the list of a field, made the first time it is asked for as the standard object's list; a field with levels starts as level 1's.</summary>
        public ObjectList<T> GetList<T>(object owner, string field)
        {
            if (!values.TryGetValue(field, out var list))
            {
                var made = new ObjectList<T>(StandardEntries(ReadStandard(owner, field, 0) ?? ReadStandard(owner, field, 1)));
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

        /// <summary>Gives the reference of a field, made empty the first time it is asked for.</summary>
        public ObjectRef<T> GetRef<T>(string field)
        {
            if (!values.TryGetValue(field, out var reference))
            {
                reference = new ObjectRef<T>();
                values[field] = reference;
            }

            return (ObjectRef<T>)reference!;
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

        private static string? ReadStandard(object owner, string field, int level)
        {
            var read = StandardValues.Read;
            var id = StandardIdOf(owner.GetType());
            return read == null || id == null ? null : read(id, field, level);
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

        /// <summary>The id of the nearest class in the base chain that is a standard object; null when none is.</summary>
        private static string? StandardIdOf(Type? type)
        {
            while (type != null)
            {
                var standard = (StandardAttribute?)Attribute.GetCustomAttribute(type, typeof(StandardAttribute), false);
                if (standard != null)
                {
                    return standard.Id;
                }

                type = type.BaseType;
            }

            return null;
        }
    }
}
