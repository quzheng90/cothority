# KyberJS

Javascript implementation of [Kyber interfaces](https://github.com/dedis/kyber/blob/master/group.go)

1. **This is developmental, and not ready for protecting production data.**
2. **This is not a constant time implementation, and likely has timing side channels that can be attacked.**

## Usage

In the browser:

The bundle is compiled using the command:

```
npm run bundle
```

Check index.html for a browser-based usage

In NodeJS:

```js
import kyber from "@dedis/kyber";
import { newCurve } from "@dedis/kyber/curve";
...
```

## Dev Setup

The simplest way to use a kyber development version in an app and being able to 
add debug-lines and change the code is to add the following to your
`tsconfig.json`:

```json
{
  "compilerOptions": {
    "paths": {
      "@dedis/kyber": [
        "../cothority/external/js/kyber/src",
        "node_modules/@dedis/kyber/*"
      ],
      "@dedis/kyber/*": [
        "../cothority/external/js/kyber/src/*",
        "node_modules/@dedis/kyber/*"
      ]
    }
  }
}
```

This will look for the cothority-sources in the parent directory of your app and
include those. If it doesn't find them, it will use the sources found in the `node_modules`
directory.

It is important that the cothority-repository is in the parent directory, else
typescript will try to include it in the compilation.

Also, the cothority/external/js/kyber-sources need to have all the libraries installed with
`npm ci`, else the compilation will fail.

### Old way

```
git clone https://github.com/dedis/cothority
cd cothority/external/js/kyber
npm run link

cd $WORK_DIR
npm link @dedis/kyber
```

## Browser Build

`npm run build` will transpile the typescript files of the _src_ folder into _dist_ and
`npm run bundle` will pack everything inside a minimalistic bundle again in _dist_

## Running Tests

Execute `npm test` to run the unit tests and get the coverage

## Generate Documentation

Execute `npm run doc` to generate the documentation and browse doc/index.html

# Releases

Please have a look at [PUBLISH.md](../../../PUBLISH.md) for how to create
 releases.
