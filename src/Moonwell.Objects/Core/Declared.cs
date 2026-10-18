using System;
using System.Collections.Generic;

namespace Moonwell.Objects
{
    /// <summary>The values a declared object puts on its fields while its constructors run, on .NET.</summary>
    /// @CSharpLua.Ignore
    public sealed class Declared
    {
        private readonly Dictionary<string, object?> values = new Dictionary<string, object?>();

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

        /// <summary>Puts a value on a field.</summary>
        public void Put<T>(string field, T value)
        {
            values[field] = value;
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

        /// <summary>Gives the list of a field, made empty the first time it is asked for.</summary>
        public ObjectList<T> GetList<T>(string field)
        {
            if (!values.TryGetValue(field, out var list))
            {
                list = new ObjectList<T>();
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

        private static string? ReadStandard(object owner, string field, int level)
        {
            var read = StandardValues.Read;
            var id = StandardIdOf(owner.GetType());
            return read == null || id == null ? null : read(id, field, level);
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
