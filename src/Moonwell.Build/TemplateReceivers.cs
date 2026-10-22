using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using Microsoft.CodeAnalysis.Operations;
using Microsoft.CodeAnalysis.Text;

namespace Moonwell.Build
{
    /// <summary>
    /// Writes <c>(this)</c> as the receiver of an instance property with a <c>@CSharpLua.Get</c> or <c>@CSharpLua.Set</c> template
    /// that is read or put with none, through <c>this.</c> or through <c>base.</c> (C20): CSharp.lua fills a property template's
    /// <c>{this}</c> only from a receiver it reads as an expression, and stops with a NullReferenceException, naming no file, on a
    /// bare name or a name after <c>this.</c> or <c>base.</c>. The kind bases' <c>Default</c> in the build's stubs is such a property
    /// (<see cref="ObjectStubs"/>), so <c>Default.Charges</c> in a declared class's own method becomes <c>(this).Default.Charges</c>,
    /// which CSharp.lua writes as <c>Default.Charges</c> read from <c>this</c>. A <c>base.</c> receiver is rewritten only where
    /// <c>(this).</c> reads the same property.
    /// </summary>
    internal static class TemplateReceivers
    {
        private const string This = "(this)";

        /// <summary>Adds the edits of a file to <paramref name="edits"/>.</summary>
        internal static void Edits(SyntaxNode root, SemanticModel model, List<FloatEdit> edits)
        {
            // A primary constructor's operations may hold what its members' hold too: each receiver is written once.
            var done = new HashSet<int>();
            foreach (var operation in RunTimeFloats.Roots(root, model))
            {
                foreach (var reference in operation.DescendantsAndSelf().OfType<IPropertyReferenceOperation>())
                {
                    if (reference.Property.IsStatic || !IsTemplate(reference.Property)
                        || reference.Instance is not IInstanceReferenceOperation { ReferenceKind: InstanceReferenceKind.ContainingTypeInstance } instance)
                    {
                        continue;
                    }

                    var name = reference.Property.Name;
                    if (instance.IsImplicit && reference.Syntax is IdentifierNameSyntax bare && done.Add(bare.SpanStart))
                    {
                        // The name is replaced, not written before: a bracket another edit opens at the same place (an int
                        // made a float, RunTimeFloats) then goes before the receiver, as an insert goes before a replacement.
                        edits.Add(new FloatEdit(bare.Span, This + "." + bare.Identifier.Text, 0, name + " -> " + This + "." + name));
                    }
                    else if (reference.Syntax is MemberAccessExpressionSyntax { Expression: ThisExpressionSyntax or BaseExpressionSyntax } access
                        && (access.Expression is ThisExpressionSyntax || ReadsTheSame(access, reference.Property, model))
                        && done.Add(access.Expression.SpanStart))
                    {
                        edits.Add(new FloatEdit(access.Expression.Span, This, 0, access.Expression + "." + name + " -> " + This + "." + name));
                    }
                }
            }
        }

        // A property CSharp.lua writes by its template: one whose declaration carries @CSharpLua.Get or @CSharpLua.Set.
        private static bool IsTemplate(IPropertySymbol property)
        {
            foreach (var reference in property.DeclaringSyntaxReferences)
            {
                var comments = reference.GetSyntax().GetLeadingTrivia().ToFullString();
                if (comments.Contains("@CSharpLua.Get", StringComparison.Ordinal) || comments.Contains("@CSharpLua.Set", StringComparison.Ordinal))
                {
                    return true;
                }
            }

            return false;
        }

        // Whether (this).Name in place of base.Name reads the same property: not one a class between hides.
        private static bool ReadsTheSame(MemberAccessExpressionSyntax access, IPropertySymbol property, SemanticModel model)
        {
            var speculative = SyntaxFactory.ParseExpression(This + "." + access.Name);
            var symbol = model.GetSpeculativeSymbolInfo(access.SpanStart, speculative, SpeculativeBindingOption.BindAsExpression).Symbol;
            return SymbolEqualityComparer.Default.Equals(symbol, property);
        }
    }
}
