using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Operations;
using Microsoft.CodeAnalysis.Text;

namespace Moonwell.Build
{
    /// <summary>
    /// Writes <c>.ToString()</c> after a nullable value joined to a string (C21): CSharp.lua writes such a value, in a
    /// concatenation, after <c>+=</c> on a string, or in an interpolation hole with no width, as <c>n:ToString()</c>, a method of
    /// the Lua value, and a number or a boolean has methods only where CSharp.lua's core gives them through the debug library,
    /// which the game does not have; nil never has. <c>"n=" + n</c> becomes <c>"n=" + n.ToString()</c>, which CSharp.lua writes
    /// as <c>System.toString(n)</c>, and which C# defines as the same text: the value's, or <c>""</c> for null. A hole with a
    /// format and no width, <c>{n:F1}</c>, becomes <c>{n?.ToString("F1")}</c> (null gives <c>""</c> in the hole), or
    /// <c>{n.ToString()}</c> for a type with no <c>ToString(string)</c>, whose format C# leaves out. A nullable enum is left: CSharp.lua
    /// writes it with <c>System.EnumToString</c>, which gives <c>""</c> for nil.
    /// </summary>
    internal static class NullableText
    {
        /// <summary>Adds the edits of a file to <paramref name="edits"/>.</summary>
        internal static void Edits(SyntaxNode root, SemanticModel model, List<FloatEdit> edits)
        {
            // A primary constructor's operations may hold what its members' hold too: each value is rewritten once.
            var done = new HashSet<TextSpan>();
            foreach (var operation in RunTimeFloats.Roots(root, model))
            {
                foreach (var node in operation.DescendantsAndSelf())
                {
                    switch (node)
                    {
                        case IBinaryOperation { OperatorKind: BinaryOperatorKind.Add } binary when IsConcatenation(binary.Type, binary.OperatorMethod):
                            Joined(binary.LeftOperand, done, edits);
                            Joined(binary.RightOperand, done, edits);
                            break;
                        case ICompoundAssignmentOperation { OperatorKind: BinaryOperatorKind.Add } compound when IsConcatenation(compound.Type, compound.OperatorMethod):
                            Joined(compound.Value, done, edits);
                            break;
                        case IInterpolationOperation { Alignment: null } hole:
                            Interpolated(hole, done, edits);
                            break;
                    }
                }
            }
        }

        // A + that joins strings: one whose result is a string, with no operator of another type.
        private static bool IsConcatenation(ITypeSymbol? type, IMethodSymbol? method)
        {
            return type?.SpecialType == SpecialType.System_String && (method == null || method.ContainingType.SpecialType == SpecialType.System_String);
        }

        // n joined to a string becomes n.ToString().
        private static void Joined(IOperation operand, HashSet<TextSpan> done, List<FloatEdit> edits)
        {
            var value = Unconverted(operand);
            if (IsNullableValue(value.Type) && value.Syntax is ExpressionSyntax expression && done.Add(expression.Span))
            {
                Wrap(expression, ".ToString()", edits);
            }
        }

        // {n} becomes {n.ToString()}; {n:F1} becomes {n?.ToString("F1")}, or {n.ToString()} where the type has no ToString(string).
        private static void Interpolated(IInterpolationOperation hole, HashSet<TextSpan> done, List<FloatEdit> edits)
        {
            var value = Unconverted(hole.Expression);
            if (!IsNullableValue(value.Type) || value.Syntax is not ExpressionSyntax expression || hole.Syntax is not InterpolationSyntax syntax
                || !done.Add(expression.Span))
            {
                return;
            }

            if (syntax.FormatClause is not { } format)
            {
                Wrap(expression, ".ToString()", edits);
                return;
            }

            var inner = ((INamedTypeSymbol)value.Type!).TypeArguments[0];
            var call = HasFormattedToString(inner)
                ? "?.ToString(" + SymbolDisplay.FormatLiteral(format.FormatStringToken.ValueText, quote: true) + ")"
                : ".ToString()";
            Wrap(expression, call, edits);
            edits.Add(new FloatEdit(format.Span, string.Empty, 0, string.Empty));
        }

        // The value under the conversions C# makes implicitly (to object, for a concatenation).
        private static IOperation Unconverted(IOperation operation)
        {
            while (operation is IConversionOperation { IsImplicit: true } conversion)
            {
                operation = conversion.Operand;
            }

            return operation;
        }

        // A Nullable<T> of a value type other than an enum.
        private static bool IsNullableValue(ITypeSymbol? type)
        {
            return type is INamedTypeSymbol { OriginalDefinition.SpecialType: SpecialType.System_Nullable_T } nullable
                && nullable.TypeArguments[0].TypeKind != TypeKind.Enum;
        }

        private static bool HasFormattedToString(ITypeSymbol type)
        {
            return type.GetMembers("ToString").OfType<IMethodSymbol>().Any(method =>
                !method.IsStatic && method.DeclaredAccessibility == Accessibility.Public && method.Parameters.Length == 1
                && method.Parameters[0].Type.SpecialType == SpecialType.System_String);
        }

        // The call after the expression, with brackets around it where the call would bind to a part of it ((n + 1), a?.Count).
        private static void Wrap(ExpressionSyntax expression, string call, List<FloatEdit> edits)
        {
            var bare = IsWhole(expression);
            var open = bare ? string.Empty : "(";
            var close = (bare ? string.Empty : ")") + call;
            // As RunTimeFloats: at one place, the brackets that close come first, inner before outer, then the brackets that open,
            // outer before inner.
            var length = expression.Span.Length;
            if (!bare)
            {
                edits.Add(new FloatEdit(new TextSpan(expression.SpanStart, 0), open, int.MinValue + length, string.Empty));
            }

            edits.Add(new FloatEdit(new TextSpan(expression.Span.End, 0), close, int.MaxValue - length, expression + " -> " + open + expression + close));
        }

        // An expression a member access after it reads whole.
        private static bool IsWhole(ExpressionSyntax expression)
        {
            return expression.Kind() is SyntaxKind.IdentifierName or SyntaxKind.SimpleMemberAccessExpression or SyntaxKind.InvocationExpression
                or SyntaxKind.ElementAccessExpression or SyntaxKind.ParenthesizedExpression;
        }
    }
}
