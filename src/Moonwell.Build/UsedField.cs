namespace Moonwell.Build
{
    /// <summary>An object field a class of <c>Moonwell.Objects</c> declares as a property.</summary>
    /// <param name="Property">The property's name, such as <c>GoldCost</c>; the Lua reads it with <c>get&lt;Property&gt;</c>.</param>
    /// <param name="FieldId">The field's id, such as <c>uhpm</c>.</param>
    /// <param name="Access">What the property gives: <c>value</c>, <c>levels</c> (a <c>Levels&lt;T&gt;</c>), <c>list</c> (an <c>ObjectList&lt;T&gt;</c>) or <c>ref</c> (an <c>ObjectRef&lt;T&gt;</c>).</param>
    public sealed record UsedField(string Property, string FieldId, string Access);
}
