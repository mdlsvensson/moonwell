using System.Reflection;
using System.Reflection.Emit;

namespace Moonwell.Build
{
    // Finds a call to a declared field's setter from a method that is not a constructor (design call 5): declared fields
    // are class defaults, set once while the class is declared. A lambda inside a constructor is compiled to a method
    // whose name holds <.ctor>, and counts as the constructor.
    internal static class SetterScan
    {
        private const string FieldAttributeName = "Moonwell.Objects.FieldAttribute";

        private static readonly Dictionary<short, OpCode> OpCodesByValue = ReadOpCodes();

        /// <summary>Throws for the first call to a declared field's setter outside a constructor.</summary>
        public static void Check(Assembly map)
        {
            const BindingFlags all = BindingFlags.Instance | BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.DeclaredOnly;
            foreach (var type in map.GetTypes().OrderBy(type => type.FullName, StringComparer.Ordinal))
            {
                var methods = new List<MethodBase>();
                methods.AddRange(type.GetMethods(all));
                methods.AddRange(type.GetConstructors(all));
                foreach (var method in methods.OrderBy(method => method.Name, StringComparer.Ordinal))
                {
                    if (method.Name == ".ctor" || method.Name.Contains("<.ctor>", StringComparison.Ordinal))
                    {
                        continue;
                    }

                    var property = FirstFieldSetterCalled(method);
                    if (property != null)
                    {
                        throw new BuildException($"{MapClasses.Describe(type)}.{method.Name} sets {property.Name}; declared fields are set only in the constructor");
                    }
                }
            }
        }

        private static PropertyInfo? FirstFieldSetterCalled(MethodBase method)
        {
            var il = method.GetMethodBody()?.GetILAsByteArray();
            if (il == null)
            {
                return null;
            }

            var typeArguments = method.DeclaringType!.IsGenericType ? method.DeclaringType.GetGenericArguments() : null;
            var methodArguments = method is MethodInfo { IsGenericMethod: true } generic ? generic.GetGenericArguments() : null;
            var position = 0;
            while (position < il.Length)
            {
                var code = il[position++];
                var key = code == 0xFE ? unchecked((short)(0xFE00 | il[position++])) : code;
                var opCode = OpCodesByValue[key];
                if (opCode == OpCodes.Call || opCode == OpCodes.Callvirt)
                {
                    var target = Resolve(method, BitConverter.ToInt32(il, position), typeArguments, methodArguments);
                    var property = target == null ? null : FieldPropertyOfSetter(target);
                    if (property != null)
                    {
                        return property;
                    }
                }

                position += OperandSize(opCode.OperandType, il, position);
            }

            return null;
        }

        private static MethodBase? Resolve(MethodBase method, int token, Type[]? typeArguments, Type[]? methodArguments)
        {
            try
            {
                return method.Module.ResolveMethod(token, typeArguments, methodArguments);
            }
            catch (ArgumentException)
            {
                // A token that names a generic argument of an open type has nothing to resolve to here.
                return null;
            }
        }

        private static PropertyInfo? FieldPropertyOfSetter(MethodBase target)
        {
            if (!target.IsSpecialName || !target.Name.StartsWith("set_", StringComparison.Ordinal))
            {
                return null;
            }

            foreach (var property in target.DeclaringType!.GetProperties(BindingFlags.Instance | BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic))
            {
                if (property.SetMethod == target && property.GetCustomAttributesData().Any(data => data.AttributeType.FullName == FieldAttributeName))
                {
                    return property;
                }
            }

            return null;
        }

        private static int OperandSize(OperandType type, byte[] il, int position)
        {
            switch (type)
            {
                case OperandType.InlineNone:
                    return 0;
                case OperandType.ShortInlineBrTarget:
                case OperandType.ShortInlineI:
                case OperandType.ShortInlineVar:
                    return 1;
                case OperandType.InlineVar:
                    return 2;
                case OperandType.InlineI8:
                case OperandType.InlineR:
                    return 8;
                case OperandType.InlineSwitch:
                    return 4 + (4 * BitConverter.ToInt32(il, position));
                default:
                    return 4;
            }
        }

        private static Dictionary<short, OpCode> ReadOpCodes()
        {
            var table = new Dictionary<short, OpCode>();
            foreach (var field in typeof(OpCodes).GetFields(BindingFlags.Public | BindingFlags.Static))
            {
                if (field.FieldType == typeof(OpCode))
                {
                    var opCode = (OpCode)field.GetValue(null)!;
                    table[opCode.Value] = opCode;
                }
            }

            return table;
        }
    }
}
