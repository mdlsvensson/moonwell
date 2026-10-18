using System;
using System.Collections.Generic;

namespace Moonwell.Objects
{
    /// <summary>Checks a value put by field id against the type its field holds, so that a wrong one is an error naming the field.</summary>
    /// @CSharpLua.Ignore
    internal static class FieldChecks
    {
        /// <summary>Gives <paramref name="value"/> when it is of the type <paramref name="held"/> says <paramref name="field"/> holds; else throws naming the field.</summary>
        internal static object Value(string owner, IReadOnlyDictionary<string, Type> held, string field, object? value)
        {
            var type = HeldType(owner, held, field);
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

        /// <summary>Gives <paramref name="value"/> when it is of the type one level of <paramref name="field"/> holds; else throws naming the field.</summary>
        internal static object LevelValue(string owner, IReadOnlyDictionary<string, Type> held, string field, int level, object? value)
        {
            var type = HeldType(owner, held, field);
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

        private static Type HeldType(string owner, IReadOnlyDictionary<string, Type> held, string field)
        {
            if (!held.TryGetValue(field, out var type))
            {
                throw new ArgumentException(owner + " has no field " + field, "field");
            }

            return type;
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
