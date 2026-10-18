using System.Globalization;
using War3Net.Build.Object;

namespace Moonwell.Build
{
    // How a declared value is written in an object file: by the field's store type, a number, a real or a text.
    internal static class StoredValues
    {
        /// <summary>The type and value to write for a field's declared value.</summary>
        /// <param name="owner">The declared class, for messages.</param>
        /// <param name="field">The field.</param>
        /// <param name="value">The declared value: <c>int</c>, <c>float</c>, <c>bool</c> or <c>string</c>; for a list the class names (<c>IReadOnlyList&lt;string&gt;</c>), for a reference the class name.</param>
        /// <param name="idOf">Gives the object id of a class name, or text that is an id; throws for a name that is none.</param>
        public static (ObjectDataType Type, object Value) Of(string owner, StandardData.ObjectField field, object value, Func<string, string> idOf)
        {
            if (value is IReadOnlyList<string> names)
            {
                return (ObjectDataType.String, string.Join(",", names.Select(idOf)));
            }

            if (field.Type == "objectRef" && value is string name)
            {
                return (ObjectDataType.String, idOf(name));
            }

            switch (field.Store)
            {
                case "int":
                    return (ObjectDataType.Int, AsInt(owner, field, value));
                case "real":
                    return (ObjectDataType.Real, AsReal(owner, field, value));
                case "unreal":
                    return (ObjectDataType.Unreal, AsReal(owner, field, value));
                default:
                    return (ObjectDataType.String, AsText(value));
            }
        }

        private static int AsInt(string owner, StandardData.ObjectField field, object value)
        {
            switch (value)
            {
                case int number:
                    return number;
                case bool flag:
                    return flag ? 1 : 0;
                case float real:
                    return (int)real;
                case string text when int.TryParse(text, NumberStyles.Integer, CultureInfo.InvariantCulture, out var parsed):
                    return parsed;
                default:
                    throw new BuildException($"{owner}: {field.Id} is a whole number in the object file, and \"{value}\" is not one");
            }
        }

        private static float AsReal(string owner, StandardData.ObjectField field, object value)
        {
            switch (value)
            {
                case float real:
                    return real;
                case int number:
                    return number;
                case bool flag:
                    return flag ? 1f : 0f;
                case string text when float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var parsed):
                    return parsed;
                default:
                    throw new BuildException($"{owner}: {field.Id} is a real number in the object file, and \"{value}\" is not one");
            }
        }

        private static string AsText(object value)
        {
            switch (value)
            {
                case string text:
                    return text;
                case bool flag:
                    return flag ? "1" : "0";
                case float real:
                    return real.ToString("R", CultureInfo.InvariantCulture);
                default:
                    return Convert.ToString(value, CultureInfo.InvariantCulture) ?? string.Empty;
            }
        }
    }
}
