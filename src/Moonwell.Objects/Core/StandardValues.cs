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

        /// <summary>
        /// Turns the text of a value into its type; null gives the default of the type. A text that is not a number of its type
        /// reads as the build's Lua side reads it: a decimal truncated for an <c>int</c>, else 0. A choice's text gives its enum's
        /// member, the other spellings of the game's data too (<c>aliv</c> is <see cref="Targets.Alive"/>).
        /// </summary>
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

#if !MOONWELL_LUA
            if (typeof(T).IsEnum)
            {
                return (T)Enum.ToObject(typeof(T), ChoiceTexts.Number(typeof(T), text));
            }
#endif

            if (typeof(T) == typeof(int))
            {
                if (int.TryParse(text, NumberStyles.Integer, CultureInfo.InvariantCulture, out var whole))
                {
                    return (T)(object)whole;
                }

                return (T)(object)(float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var truncated) ? (int)truncated : 0);
            }

            if (typeof(T) == typeof(float))
            {
                return (T)(object)(float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var parsed) ? parsed : 0f);
            }

            if (typeof(T) == typeof(bool))
            {
                return (T)(object)(text == "1");
            }

            throw new NotSupportedException("StandardValues.Parse: " + typeof(T).Name + " is not a field type");
        }
    }
}
