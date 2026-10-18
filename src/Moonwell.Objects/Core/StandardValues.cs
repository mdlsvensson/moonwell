using System;
using System.Globalization;

namespace Moonwell.Objects
{
    /// <summary>Where the values of the game's standard objects come from while a class is made on .NET.</summary>
    /// @CSharpLua.Ignore
    public static class StandardValues
    {
        /// <summary>Gives the game's value as text for a standard id, a field id and a level (0 when the field has none); null when it has none. The build sets it.</summary>
        public static Func<string, string, int, string?>? Read { get; set; }

        /// <summary>Turns the text of a value into its type; null gives the default of the type.</summary>
        internal static T Parse<T>(string? text)
        {
            if (typeof(T) == typeof(string))
            {
                return (T)(object?)text!;
            }

            if (text == null)
            {
                return default!;
            }

            if (typeof(T) == typeof(int))
            {
                return (T)(object)int.Parse(text, CultureInfo.InvariantCulture);
            }

            if (typeof(T) == typeof(float))
            {
                return (T)(object)float.Parse(text, CultureInfo.InvariantCulture);
            }

            if (typeof(T) == typeof(bool))
            {
                return (T)(object)(text == "1");
            }

            throw new NotSupportedException("StandardValues.Parse: " + typeof(T).Name + " is not a field type");
        }
    }
}
