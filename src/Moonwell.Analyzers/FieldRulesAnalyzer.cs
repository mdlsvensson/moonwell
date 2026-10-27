using System;
using System.Collections.Generic;
using System.Collections.Immutable;
using System.Globalization;
using System.Linq;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.CodeAnalysis.Operations;

namespace Moonwell.Analyzers;

/// <summary>
/// Rejects a constant a map class puts outside World Editor's limits (MW0009): in an assignment to a field's property, in the
/// arguments of <c>Levels(...)</c>, as one value for every level (<c>Cooldown = 3900f;</c>, which the build checks at each level
/// and names at level 1), or in <c>Set("id", ...)</c>. The text is the one the build gives for the same value
/// (<c>Moonwell.Objects.RuleChecks</c>), without its "file:line: " prefix. Only a constant is checked; a computed value is
/// left to the build.
/// </summary>
/// <remarks>
/// The rules are read from the attributes of the property the field belongs to; a shared field, which has no property,
/// has them on its kind base's <c>[SharedField]</c>, and is checked where it is declared: set by id, or by its live member
/// on the object itself in its class's constructor (elsewhere that member is the live value). A hero's most life, most
/// mana and starting mana set by their live members are the totals it starts with, whose base fields the build checks. A <c>[BeyondLimits]</c> on the class that
/// holds the assignment, or on one of its base classes, lifts a range, a length or a number of entries but never a negative
/// where none is allowed, an empty required name or a choice that is no member (the build names the class that is being
/// constructed instead; the analyzer only sees the class that holds the assignment). The analyzer also reports a
/// <c>[BeyondLimits]</c> name that is no field property of the class or its bases, which lifts nothing.
/// </remarks>
[DiagnosticAnalyzer(LanguageNames.CSharp)]
public sealed class FieldRulesAnalyzer : DiagnosticAnalyzer
{
    private const string Lift = "; [BeyondLimits] goes past it";

    public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics => Rules.Objects;

    public override void Initialize(AnalysisContext context)
    {
        context.ConfigureGeneratedCodeAnalysis(GeneratedCodeAnalysisFlags.None);
        context.EnableConcurrentExecution();
        context.RegisterOperationAction(Assignment, OperationKind.SimpleAssignment);
        context.RegisterOperationAction(Call, OperationKind.Invocation);
        context.RegisterSymbolAction(Declaration, SymbolKind.NamedType);
    }

    /// <summary>The rules of one field as one class has them.</summary>
    private sealed class Field
    {
        public string Property = string.Empty;
        public string Id = string.Empty;
        public string Label = string.Empty;
        public bool Lifted;
        public bool HasLimits;
        public double Min = double.NegativeInfinity;
        public double Max = double.PositiveInfinity;
        public bool NonNegative;
        public bool Required;
        public int? MaxLength;
        public int? MaxEntries;

        // A whole-number field that its live member puts as a float: the build truncates the value first.
        public bool Whole;
    }

    // The fields whose live member is a float, which their setter truncates.
    private static readonly HashSet<string> WholeFields = new() { "uhpm", "umpm", "umpi" };

    // The fields a hero's live member puts as a total or not at all, which the build checks as base fields.
    private static readonly HashSet<string> HeroTotals = new() { "uhpm", "umpm", "umpi", "ulev" };

    private static void Assignment(OperationAnalysisContext context)
    {
        var assignment = (ISimpleAssignmentOperation)context.Operation;
        if (assignment.Target is IPropertyReferenceOperation reference
            && EnclosingType(context.ContainingSymbol) is { } holder
            && (ReadField(reference.Property, holder) ?? SharedAssignment(context, reference, holder)) is { } field)
        {
            Value(context, field, null, assignment.Value);
        }
    }

    /// <summary>The shared field a live member declares: set on the object itself in its class's constructor; null anywhere else, and for a hero's total.</summary>
    private static Field? SharedAssignment(OperationAnalysisContext context, IPropertyReferenceOperation reference, INamedTypeSymbol holder)
    {
        if (context.ContainingSymbol is not IMethodSymbol { MethodKind: MethodKind.Constructor, IsStatic: false }
            || reference.Instance is not IInstanceReferenceOperation
            || reference.Property.ContainingType?.ContainingNamespace is not { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true }
            || InLambda(reference)
            || SharedFieldOf(holder, null, reference.Property.Name) is not { } shared)
        {
            return null;
        }

        var field = ReadShared(shared, holder);
        if (HeroTotals.Contains(field.Id) && IsHero(holder))
        {
            return null;
        }

        field.Whole = WholeFields.Contains(field.Id);
        return field;
    }

    private static void Call(OperationAnalysisContext context)
    {
        var call = (IInvocationOperation)context.Operation;
        var method = call.TargetMethod;
        if (method.Name != "Set" || !IsObjects(method.ContainingType, "") || method.Parameters.Length is not (2 or 3)
            || method.Parameters[0].Type.SpecialType != SpecialType.System_String
            || method.Parameters[method.Parameters.Length - 1].Type.SpecialType != SpecialType.System_Object
            || call.Arguments.Length != method.Parameters.Length)
        {
            return;
        }

        if (call.Arguments[0].Value.ConstantValue is not { HasValue: true, Value: string id })
        {
            return;
        }

        int? level = null;
        if (call.Arguments.Length == 3)
        {
            if (call.Arguments[1].Value.ConstantValue is not { HasValue: true, Value: int at })
            {
                return;
            }

            level = at;
        }

        var holder = EnclosingType(context.ContainingSymbol);
        var property = holder == null ? null : PropertyOf(holder, id);
        var field = holder == null ? null
            : property != null ? ReadField(property, holder)
            : SharedFieldOf(holder, id, null) is { } shared ? ReadShared(shared, holder)
            : null;
        if (field != null)
        {
            Value(context, field, level, call.Arguments[call.Arguments.Length - 1].Value);
        }
    }

    /// <summary>
    /// Reports the first value of a put that breaks a rule: the constant itself, the first of the constants of a
    /// <c>Levels(...)</c>, or one value for every level at level 1, the first level the build checks it at.
    /// </summary>
    private static void Value(OperationAnalysisContext context, Field field, int? level, IOperation value)
    {
        value = Unbox(value);
        if (level == null && LevelValues(value) is { } levels)
        {
            for (var i = 0; i < levels.Length; i++)
            {
                if (Constant(context, field, i + 1, levels[i]))
                {
                    return;
                }
            }

            return;
        }

        if (level == null && OneForEveryLevel(value) is { } every)
        {
            Constant(context, field, 1, every);
            return;
        }

        Constant(context, field, level, value);
    }

    /// <summary>The value of <c>Levels&lt;T&gt;</c>'s implicit conversion, one value for every level; null when the operation is no such conversion.</summary>
    private static IOperation? OneForEveryLevel(IOperation operation)
    {
        return operation is IConversionOperation { IsImplicit: true, OperatorMethod: { } conversion } levels
            && IsObjects(conversion.ContainingType?.OriginalDefinition, "Levels")
            ? levels.Operand
            : null;
    }

    private static bool Constant(OperationAnalysisContext context, Field field, int? level, IOperation value)
    {
        if (!value.ConstantValue.HasValue || Violation(field, value.Type, value.ConstantValue.Value) is not { } broken)
        {
            return false;
        }

        context.ReportDiagnostic(Diagnostic.Create(Rules.FieldRule, value.Syntax.GetLocation(), Message(field, level, broken.Value, broken.Allowed)));
        return true;
    }

    /// <summary>The same text as <c>FieldRuleException.MessageOf</c>.</summary>
    private static string Message(Field field, int? level, string value, string allowed)
    {
        var at = level.HasValue ? " at level " + level.Value.ToString(CultureInfo.InvariantCulture) : string.Empty;
        var named = field.Label.Length > 0 ? field.Label + " (" + field.Id + ")" : field.Id;
        return field.Property + at + " is " + value + "; " + named + " allows " + allowed;
    }

    /// <summary>The value an operation gives, without the boxing of an argument of type object.</summary>
    private static IOperation Unbox(IOperation operation)
    {
        while (operation is IConversionOperation { IsImplicit: true, Type.SpecialType: SpecialType.System_Object } conversion)
        {
            operation = conversion.Operand;
        }

        return operation;
    }

    /// <summary>The values given to <c>Levels(...)</c>, in order; null when the operation is no such call or the values are no list written out.</summary>
    private static ImmutableArray<IOperation>? LevelValues(IOperation operation)
    {
        if (operation is not IInvocationOperation { TargetMethod.Name: "Levels" } call
            || !IsObjects(call.TargetMethod.ReturnType.OriginalDefinition as INamedTypeSymbol, "Levels")
            || call.Arguments.Length != 1)
        {
            return null;
        }

        var list = call.Arguments[0].Value;
        while (list is IConversionOperation { IsImplicit: true } conversion)
        {
            list = conversion.Operand;
        }

        return list is IArrayCreationOperation { Initializer: { } initializer } ? initializer.ElementValues : null;
    }

    /// <summary>What a constant breaks, as the value and what is allowed, in the order the build checks; null when it is allowed.</summary>
    private static (string Value, string Allowed)? Violation(Field field, ITypeSymbol? type, object? value)
    {
        if (type is { TypeKind: TypeKind.Enum } && value != null)
        {
            return Choice(type, value);
        }

        switch (value)
        {
            case int whole:
                return Number(field, whole, false, whole.ToString(CultureInfo.InvariantCulture), string.Empty);
            case float real when field.Whole:
                var truncated = (int)real;
                return Number(field, truncated, false, truncated.ToString(CultureInfo.InvariantCulture), string.Empty);
            case float real:
                return Number(field, real, true, real.ToString("R", CultureInfo.InvariantCulture), string.Empty);
            case string text:
                return Text(field, text);
            case null:
                return field.Required ? ("null", "any text that is not empty") : null;
            default:
                return null;
        }
    }

    // A float is compared with the limits narrowed to float, as the build does: 0.01f is below the double 0.01.
    private static (string, string)? Number(Field field, double number, bool single, string shown, string each)
    {
        double min = single ? (float)field.Min : field.Min;
        double max = single ? (float)field.Max : field.Max;
        if (field.NonNegative && number < 0)
        {
            var allowed = field.Lifted ? "0 or more, even with [BeyondLimits]"
                : field.HasLimits ? Range(field.Min, field.Max) + each
                : "0 or more";
            return (shown, allowed);
        }

        if (field.HasLimits && !field.Lifted && !(number >= min && number <= max))
        {
            return (shown, Range(field.Min, field.Max) + each + Lift);
        }

        return null;
    }

    private static (string, string)? Text(Field field, string text)
    {
        var shown = "\"" + text + "\"";
        if (field.Required && text.Length == 0)
        {
            return (shown, "any text that is not empty");
        }

        if (field.MaxLength is { } length && !field.Lifted && text.Length > length)
        {
            return (shown + " (" + Count(text.Length) + " characters)", "at most " + Count(length) + " characters" + Lift);
        }

        var entries = text.Length == 0 ? new string[0] : text.Split(',');
        if (field.MaxEntries is { } most && !field.Lifted && entries.Length > most)
        {
            return (shown + " (" + Count(entries.Length) + " entries)", "at most " + Count(most) + " entries" + Lift);
        }

        if (field.HasLimits || field.NonNegative)
        {
            // A number list held as text: each number is checked; a part that is no number is left to the game.
            foreach (var entry in entries)
            {
                if (double.TryParse(entry.Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out var number)
                    && Number(field, number, false, shown, " for each number") is { } broken)
                {
                    return broken;
                }
            }
        }

        return null;
    }

    private static (string, string)? Choice(ITypeSymbol type, object value)
    {
        var number = ToLong(value);
        var flags = type.GetAttributes().Any(a => a.AttributeClass is { Name: "FlagsAttribute", ContainingNamespace: { Name: "System", ContainingNamespace.IsGlobalNamespace: true } });
        var members = type.GetMembers().OfType<IFieldSymbol>().Where(m => m.HasConstantValue).Select(m => ToLong(m.ConstantValue!)).ToList();
        bool known;
        if (flags)
        {
            long all = 0;
            foreach (var member in members)
            {
                all |= member;
            }

            known = (number & ~all) == 0;
        }
        else
        {
            known = members.Contains(number);
        }

        return known ? null : (number.ToString(CultureInfo.InvariantCulture), "only the values of " + type.Name + (flags ? " and their combinations" : string.Empty));
    }

    private static long ToLong(object value) => value is ulong big ? unchecked((long)big) : Convert.ToInt64(value, CultureInfo.InvariantCulture);

    private static string Range(double min, double max)
    {
        if (double.IsNegativeInfinity(min))
        {
            return "at most " + Number(max);
        }

        if (double.IsPositiveInfinity(max))
        {
            return Number(min) + " or more";
        }

        return Number(min) + " to " + Number(max);
    }

    private static string Number(double number) => number.ToString("R", CultureInfo.InvariantCulture);

    private static string Count(int count) => count.ToString(CultureInfo.InvariantCulture);

    /// <summary>The rules of a field's property, as the class that holds the assignment has them; null when the property is no field.</summary>
    private static Field? ReadField(IPropertySymbol property, INamedTypeSymbol holder)
    {
        var field = FieldAttributeOf(property);
        if (field == null)
        {
            return null;
        }

        var result = new Field
        {
            Property = ClassName(holder) + "." + property.Name,
            Id = (string)field.ConstructorArguments[0].Value!,
            Lifted = BeyondLimitsOf(holder).Contains(property.Name),
        };
        foreach (var named in field.NamedArguments)
        {
            if (named.Key == "Label" && named.Value.Value is string label)
            {
                result.Label = label;
            }
        }

        foreach (var attribute in property.GetAttributes())
        {
            var arguments = attribute.ConstructorArguments;
            if (IsObjects(attribute.AttributeClass, "LimitsAttribute") && arguments.Length == 2)
            {
                result.HasLimits = true;
                result.Min = Convert.ToDouble(arguments[0].Value, CultureInfo.InvariantCulture);
                result.Max = Convert.ToDouble(arguments[1].Value, CultureInfo.InvariantCulture);
            }
            else if (IsObjects(attribute.AttributeClass, "NonNegativeAttribute"))
            {
                result.NonNegative = true;
            }
            else if (IsObjects(attribute.AttributeClass, "RequiredAttribute"))
            {
                result.Required = true;
            }
            else if (IsObjects(attribute.AttributeClass, "MaxLengthAttribute") && arguments.Length == 1)
            {
                result.MaxLength = (int)arguments[0].Value!;
            }
            else if (IsObjects(attribute.AttributeClass, "MaxEntriesAttribute") && arguments.Length == 1)
            {
                result.MaxEntries = (int)arguments[0].Value!;
            }
        }

        return result;
    }

    // Code in a lambda or a local function may run later in the game, where a shared member is the live value.
    private static bool InLambda(IOperation operation)
    {
        for (var parent = operation.Parent; parent != null; parent = parent.Parent)
        {
            if (parent is IAnonymousFunctionOperation or ILocalFunctionOperation)
            {
                return true;
            }
        }

        return false;
    }

    /// <summary>The rules of a shared field as the class that holds the put has them, from its <c>[SharedField]</c>.</summary>
    private static Field ReadShared(AttributeData shared, INamedTypeSymbol holder)
    {
        var member = (string)shared.ConstructorArguments[1].Value!;
        var result = new Field
        {
            Property = ClassName(holder) + "." + member,
            Id = (string)shared.ConstructorArguments[0].Value!,
            Lifted = BeyondLimitsOf(holder).Contains(member),
        };
        var min = double.NegativeInfinity;
        var max = double.PositiveInfinity;
        foreach (var named in shared.NamedArguments)
        {
            switch (named.Key)
            {
                case "Label" when named.Value.Value is string label:
                    result.Label = label;
                    break;
                case "Min" when named.Value.Value != null:
                    min = Convert.ToDouble(named.Value.Value, CultureInfo.InvariantCulture);
                    break;
                case "Max" when named.Value.Value != null:
                    max = Convert.ToDouble(named.Value.Value, CultureInfo.InvariantCulture);
                    break;
                case "HasLimits" when named.Value.Value is bool limited:
                    result.HasLimits = limited;
                    break;
                case "NonNegative" when named.Value.Value is bool nonNegative:
                    result.NonNegative = nonNegative;
                    break;
                case "Required" when named.Value.Value is bool required:
                    result.Required = required;
                    break;
            }
        }

        // As the build: the range is read only when the attribute says the field has one.
        if (result.HasLimits)
        {
            result.Min = min;
            result.Max = max;
        }

        return result;
    }

    /// <summary>The <c>[SharedField]</c> on the class or its bases with this id, or for this live member; null when none is.</summary>
    private static AttributeData? SharedFieldOf(INamedTypeSymbol type, string? id, string? member)
    {
        for (var parent = type; parent != null; parent = parent.BaseType)
        {
            foreach (var attribute in parent.GetAttributes())
            {
                if (IsObjects(attribute.AttributeClass, "SharedFieldAttribute") && attribute.ConstructorArguments.Length == 2
                    && attribute.ConstructorArguments[0].Value is string sharedId && attribute.ConstructorArguments[1].Value is string sharedMember
                    && (id == null || sharedId == id) && (member == null || sharedMember == member))
                {
                    return attribute;
                }
            }
        }

        return null;
    }

    // A class that inherits Moonwell's Hero.
    private static bool IsHero(INamedTypeSymbol type)
    {
        for (var parent = (INamedTypeSymbol?)type; parent != null; parent = parent.BaseType)
        {
            if (parent.Name == "Hero" && parent.ContainingNamespace is { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true })
            {
                return true;
            }
        }

        return false;
    }

    private static AttributeData? FieldAttributeOf(IPropertySymbol property)
    {
        foreach (var attribute in property.GetAttributes())
        {
            if (IsObjects(attribute.AttributeClass, "FieldAttribute") && attribute.ConstructorArguments.Length == 1
                && attribute.ConstructorArguments[0].Value is string)
            {
                return attribute;
            }
        }

        return null;
    }

    /// <summary>The property of the class or its bases that is the field with this id; null when none is.</summary>
    private static IPropertySymbol? PropertyOf(INamedTypeSymbol type, string id)
    {
        for (var parent = type; parent != null; parent = parent.BaseType)
        {
            foreach (var property in parent.GetMembers().OfType<IPropertySymbol>())
            {
                if (FieldAttributeOf(property) is { } field && (string?)field.ConstructorArguments[0].Value == id)
                {
                    return property;
                }
            }
        }

        return null;
    }

    /// <summary>The property names that <c>[BeyondLimits]</c> on the class and its bases lift.</summary>
    private static HashSet<string> BeyondLimitsOf(INamedTypeSymbol type)
    {
        var names = new HashSet<string>();
        for (var parent = type; parent != null; parent = parent.BaseType)
        {
            foreach (var attribute in parent.GetAttributes())
            {
                if (IsObjects(attribute.AttributeClass, "BeyondLimitsAttribute"))
                {
                    foreach (var name in NamesOf(attribute))
                    {
                        if (name.Value is string text)
                        {
                            names.Add(text);
                        }
                    }
                }
            }
        }

        return names;
    }

    private static ImmutableArray<TypedConstant> NamesOf(AttributeData attribute) =>
        attribute.ConstructorArguments.Length == 1 && attribute.ConstructorArguments[0].Kind == TypedConstantKind.Array
            ? attribute.ConstructorArguments[0].Values
            : ImmutableArray<TypedConstant>.Empty;

    /// <summary>Reports a <c>[BeyondLimits]</c> name that is no field property or shared member of the class or its bases.</summary>
    private static void Declaration(SymbolAnalysisContext context)
    {
        var type = (INamedTypeSymbol)context.Symbol;
        foreach (var attribute in type.GetAttributes())
        {
            if (!IsObjects(attribute.AttributeClass, "BeyondLimitsAttribute"))
            {
                continue;
            }

            var names = NamesOf(attribute);
            var syntax = attribute.ApplicationSyntaxReference?.GetSyntax(context.CancellationToken) as AttributeSyntax;
            var arguments = syntax?.ArgumentList?.Arguments;
            for (var i = 0; i < names.Length; i++)
            {
                if (names[i].Value is string name && !IsFieldProperty(type, name) && syntax != null)
                {
                    var location = arguments != null && arguments.Value.Count == names.Length ? arguments.Value[i].GetLocation() : syntax.GetLocation();
                    var message = "[BeyondLimits] on " + ClassName(type) + " names '" + name + "', which is no field property of " + ClassName(type) + " or its base classes";
                    context.ReportDiagnostic(Diagnostic.Create(Rules.FieldRule, location, message));
                }
            }
        }
    }

    private static bool IsFieldProperty(INamedTypeSymbol type, string name)
    {
        if (SharedFieldOf(type, null, name) != null)
        {
            return true;
        }

        for (var parent = type; parent != null; parent = parent.BaseType)
        {
            foreach (var property in parent.GetMembers(name).OfType<IPropertySymbol>())
            {
                if (FieldAttributeOf(property) != null)
                {
                    return true;
                }
            }
        }

        return false;
    }

    private static INamedTypeSymbol? EnclosingType(ISymbol? symbol)
    {
        for (; symbol != null; symbol = symbol.ContainingSymbol)
        {
            if (symbol is INamedTypeSymbol type)
            {
                return type;
            }
        }

        return null;
    }

    /// <summary>The class's name as a modder writes it, without its namespace: <c>Arthas</c>, <c>Heroes.Arthas</c> for a nested class.</summary>
    private static string ClassName(INamedTypeSymbol type)
    {
        var name = type.Name;
        for (var outer = type.ContainingType; outer != null; outer = outer.ContainingType)
        {
            name = outer.Name + "." + name;
        }

        return name;
    }

    /// <summary>Whether a type is the one of Moonwell.Objects with this name; an empty name accepts any type of the namespace.</summary>
    private static bool IsObjects(INamedTypeSymbol? type, string name) =>
        type != null && (name.Length == 0 || type.Name == name)
            && type.ContainingNamespace is { Name: "Objects", ContainingNamespace: { Name: "Moonwell", ContainingNamespace.IsGlobalNamespace: true } };
}
