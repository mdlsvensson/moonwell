using System;

namespace Moonwell
{
    /// @CSharpLua.Ignore
    /// <summary>The native makes a new handle; the analyzer rejects it inside <c>Local.For</c>.</summary>
    [AttributeUsage(AttributeTargets.Method | AttributeTargets.Property | AttributeTargets.Constructor)]
    public sealed class CreatesHandleAttribute : Attribute
    {
    }

    /// @CSharpLua.Ignore
    /// <summary>The member changes the game's state; the analyzer rejects it inside <c>Local.For</c>.</summary>
    [AttributeUsage(AttributeTargets.Method | AttributeTargets.Property | AttributeTargets.Constructor)]
    public sealed class ChangesStateAttribute : Attribute
    {
    }

    /// @CSharpLua.Ignore
    /// <summary>The function changes the game inside its own <c>GetLocalPlayer()</c> block, so it desyncs wherever it
    /// is called; the analyzer rejects every call.</summary>
    [AttributeUsage(AttributeTargets.Method | AttributeTargets.Property | AttributeTargets.Constructor)]
    public sealed class DesyncsAttribute : Attribute
    {
    }

    /// @CSharpLua.Ignore
    /// <summary>
    /// The member gives a different answer, or runs, on one machine only. The analyzer rejects a call to it outside
    /// <c>Local.For</c>, and checks the body of a map method that carries it as it checks a <c>Local.For</c> block.
    /// </summary>
    [AttributeUsage(AttributeTargets.Method | AttributeTargets.Property | AttributeTargets.Constructor)]
    public sealed class MachineLocalAttribute : Attribute
    {
    }

    /// @CSharpLua.Ignore
    /// <summary>A runtime member that may be called inside <c>Local.For</c>: it reads, or changes only what one player
    /// sees or hears. On a property it covers the getter; on a class, every member.</summary>
    [AttributeUsage(AttributeTargets.Class | AttributeTargets.Method | AttributeTargets.Property | AttributeTargets.Constructor)]
    public sealed class LocalSafeAttribute : Attribute
    {
    }

    /// @CSharpLua.Ignore
    /// <summary>Turns the multiplayer analyzer off for one method, its lambdas and local functions included.</summary>
    [AttributeUsage(AttributeTargets.Method | AttributeTargets.Property | AttributeTargets.Constructor)]
    public sealed class SyncUncheckedAttribute : Attribute
    {
    }
}
