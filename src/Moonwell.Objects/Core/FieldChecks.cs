using System;
using System.Collections.Generic;
using System.Reflection;

namespace Moonwell.Objects
{
    /// <summary>Checks a value put by field id against the type its field holds, so that a wrong one is an error naming the field.</summary>
    /// @CSharpLua.Ignore
    internal static class FieldChecks
    {
        // A Dictionary under a lock, not a ConcurrentDictionary: CSharp.lua writes Lua for the generic types a file
        // uses even when its class is ignored, and the Lua runtime has no System.Collections.Concurrent.
        /// <summary>The ids of the fields each class has as properties, by class.</summary>
        private static readonly Dictionary<Type, HashSet<string>> propertyFields = new Dictionary<Type, HashSet<string>>();

        /// <summary>Gives <paramref name="value"/> when <paramref name="target"/> has <paramref name="field"/> and it is of the type <paramref name="held"/> says the field holds; else throws naming the field.</summary>
        internal static object Value(string owner, IReadOnlyDictionary<string, Type> held, object target, string field, object? value)
        {
            var type = HeldType(owner, held, target, field);
            if (value == null)
            {
                throw new ArgumentException(owner + " field " + field + " holds " + Describe(type) + ", not null", "value");
            }

            if (!type.IsInstanceOfType(value))
            {
                var hint = IsLevels(type) ? "; give the value of one level with Set(field, level, value)" : string.Empty;
                throw new ArgumentException(owner + " field " + field + " holds " + Describe(type) + ", not " + Describe(value.GetType()) + hint, "value");
            }

            return value;
        }

        /// <summary>Gives <paramref name="value"/> when <paramref name="target"/> has <paramref name="field"/> and it is of the type one level of the field holds; else throws naming the field.</summary>
        internal static object LevelValue(string owner, IReadOnlyDictionary<string, Type> held, object target, string field, int level, object? value)
        {
            var type = HeldType(owner, held, target, field);
            if (!IsLevels(type))
            {
                throw new ArgumentException(owner + " field " + field + " holds " + Describe(type) + ", which has no levels", "field");
            }

            if (level < 1)
            {
                throw new ArgumentOutOfRangeException("level", level, owner + " field " + field + ": levels count from 1");
            }

            var element = type.GetGenericArguments()[0];
            if (value == null || !element.IsInstanceOfType(value))
            {
                var given = value == null ? "null" : Describe(value.GetType());
                throw new ArgumentException(owner + " field " + field + " holds " + Describe(element) + " for each level, not " + given, "value");
            }

            return value;
        }

        // The table of a kind base holds every field of its kind, and an ability's own fields are properties only of
        // the standard classes of the abilities they belong to: a field in the table that the object's classes do
        // not have is another ability's.
        private static Type HeldType(string owner, IReadOnlyDictionary<string, Type> held, object target, string field)
        {
            if (!held.TryGetValue(field, out var type))
            {
                throw new ArgumentException(owner + " has no field " + field, "field");
            }

            if (!PropertyFieldsOf(target.GetType()).Contains(field))
            {
                throw new ArgumentException(StandardClassOf(target.GetType(), owner) + " has no field " + field + ", which other abilities have", "field");
            }

            return type;
        }

        /// <summary>The ids of the fields that the standard classes in <paramref name="type"/>'s base chain have as properties.</summary>
        private static HashSet<string> PropertyFieldsOf(Type type)
        {
            lock (propertyFields)
            {
                if (!propertyFields.TryGetValue(type, out var ids))
                {
                    propertyFields[type] = ids = ReadPropertyFields(type);
                }

                return ids;
            }
        }

        private static HashSet<string> ReadPropertyFields(Type type)
        {
            var ids = new HashSet<string>(StringComparer.Ordinal);
            for (var parent = type; parent != null; parent = parent.BaseType)
            {
                if (parent.Assembly != typeof(FieldChecks).Assembly)
                {
                    continue;
                }

                foreach (var property in parent.GetProperties(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.DeclaredOnly))
                {
                    var attribute = property.GetCustomAttribute<FieldAttribute>(false);
                    if (attribute != null)
                    {
                        ids.Add(attribute.Id);
                    }
                }
            }

            return ids;
        }

        /// <summary>The name of the nearest standard class in <paramref name="type"/>'s base chain; <paramref name="owner"/> when there is none.</summary>
        private static string StandardClassOf(Type type, string owner)
        {
            for (var parent = type; parent != null; parent = parent.BaseType)
            {
                if (parent.IsDefined(typeof(StandardAttribute), false))
                {
                    return parent.Name;
                }
            }

            return owner;
        }

        private static bool IsLevels(Type type)
        {
            return type.IsGenericType && type.GetGenericTypeDefinition() == typeof(Levels<>);
        }

        /// <summary>The name of a type as a modder writes it: <c>int</c>, <c>Levels&lt;float&gt;</c>.</summary>
        private static string Describe(Type type)
        {
            if (type == typeof(int))
            {
                return "int";
            }

            if (type == typeof(float))
            {
                return "float";
            }

            if (type == typeof(bool))
            {
                return "bool";
            }

            if (type == typeof(string))
            {
                return "string";
            }

            if (!type.IsGenericType)
            {
                return type.Name;
            }

            var arguments = type.GetGenericArguments();
            var name = type.Name.Substring(0, type.Name.IndexOf('`')) + "<";
            for (var i = 0; i < arguments.Length; i++)
            {
                name += (i > 0 ? ", " : string.Empty) + Describe(arguments[i]);
            }

            return name + ">";
        }
    }
}
