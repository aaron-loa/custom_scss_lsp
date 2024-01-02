import sys

from lsprotocol.types import ClientCapabilities
from lsprotocol.types import CompletionList
from lsprotocol.types import CompletionParams
from lsprotocol.types import InitializeParams
from lsprotocol.types import Position
from lsprotocol.types import TextDocumentIdentifier

from lsprotocol.types import TEXT_DOCUMENT_COMPLETION
from lsprotocol.types import CompletionItem
from lsprotocol.types import CompletionParams

import pytest
import pytest_lsp
from pytest_lsp import ClientServerConfig
from pytest_lsp import LanguageClient
from pytest_lsp import client_capabilities

client_capabilities = client_capabilities("visual-studio-code")

@pytest_lsp.fixture(
    config=ClientServerConfig(
        # server_command=["../drupal-lsp/drupal-lsp"],
        server_command=["./scss-lsp"],

    ),
)
async def client(lsp_client: LanguageClient):
    # Setup
    response = await lsp_client.initialize_session(
        InitializeParams(
            capabilities=client_capabilities,
            root_uri="file:///home/ron/egm/egm1/csakom",
        )
    )
    yield
    # Teardown
    await lsp_client.shutdown_session()

@pytest.mark.asyncio
async def test_completions(client: LanguageClient):
    assert 1 == 2
