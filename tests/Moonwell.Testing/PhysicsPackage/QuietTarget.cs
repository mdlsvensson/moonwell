namespace Moonwell.Testing
{
    /// <summary>A unit component that is an IMissileTarget but writes no OnStruck, so the package must not call one: in Lua the interface's empty default is not there.</summary>
    public sealed class QuietTarget : UnitComponent, IMissileTarget
    {
    }
}
